package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/valkey-io/valkey-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
	botpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	botapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	scrapperapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/cache"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/clients"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/helpserv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/server"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/orm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const shutdownTimeout = 5 * time.Second

type repositories struct {
	chat         repository.ChatRepository
	subscription repository.SubscriptionRepository
	outbox       repository.NotificationOutboxRepository
	close        func()
}

type scrapperRuntime struct {
	logger       *slog.Logger
	httpServer   *http.Server
	grpcServer   *grpc.Server
	listener     net.Listener
	grpcPort     int
	grpcClient   *grpc.ClientConn
	kafkaWriter  *kafka.Writer
	scheduler    gocron.Scheduler
	closeStorage func()
	closeCache   func()
}

func main() {
	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))

	if err := run(logger); err != nil {
		logger.Error("scrapper stopped with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	reliabilityCfg, err := infrastructure.NewReliabilityConfig()
	if err != nil {
		return fmt.Errorf("can't load reliabilty config: %w", err)
	}

	app, err := newScrapperRuntime(ctx, cfg, reliabilityCfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		err = errors.Join(err, app.Close(shutdownCtx))
	}()

	return app.Run(ctx)
}

func newScrapperRuntime(
	ctx context.Context, cfg infrastructure.Config, rcfg infrastructure.ReliabiltyConfig, logger *slog.Logger,
) (*scrapperRuntime, error) {
	var err error
	app := &scrapperRuntime{
		logger:     logger,
		grpcPort:   cfg.ScrapperGRPCPort,
		grpcServer: grpc.NewServer(),
	}
	defer func() {
		if err == nil {
			return
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		err = errors.Join(err, app.Close(shutdownCtx))
	}()

	app.listener, err = newTCPListener(ctx, cfg.ScrapperGRPCPort)
	if err != nil {
		return nil, fmt.Errorf("can't create tcp listener: %w", err)
	}

	botPort, grpcClient, kafkaWriter, err := newBotPort(cfg, rcfg, logger)
	if err != nil {
		return nil, err
	}
	app.grpcClient = grpcClient
	app.kafkaWriter = kafkaWriter

	repos, err := newRepositories(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("can't init repository: %w", err)
	}
	app.closeStorage = repos.close

	baseScrapperPort := adapters.NewScrapperAdapter(logger, repos.chat, repos.subscription)

	vkClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.ValkeyClusterAddresse(),
		Password:    cfg.ValkeyPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("can't init valkey client: %w", err)
	}
	app.closeCache = vkClient.Close
	linksCache := cache.NewValkeyLinksCache(logger, vkClient, time.Duration(cfg.ValkeyTTLSeconds)*time.Second)
	scrapperPort := adapters.NewScrapperCachedAdapter(logger, baseScrapperPort, linksCache)

	app.httpServer = newScrapperHTTPServer(cfg, logger, scrapperPort)
	pb.RegisterScrapperServer(app.grpcServer, server.NewScrapperGRPCServe(logger, scrapperPort))

	app.scheduler, err = newScheduler(ctx, cfg, rcfg, logger, botPort, repos.outbox, repos.subscription)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func newTCPListener(ctx context.Context, port int) (net.Listener, error) {
	var lisConf net.ListenConfig
	return lisConf.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
}

func newScheduler(
	ctx context.Context,
	cfg infrastructure.Config,
	rcfg infrastructure.ReliabiltyConfig,
	logger *slog.Logger,
	botPort ports.BotPort,
	notificationOutboxRepo repository.NotificationOutboxRepository,
	subscriptionRepo repository.SubscriptionRepository,
) (gocron.Scheduler, error) {
	linkStatePort := adapters.NewLinkStateAdapter(
		logger,
		clients.NewGitHubClient(logger, rcfg),
		clients.NewStackoverflowClient(logger, rcfg),
	)

	sched, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("can't init scheduler: %w", err)
	}

	const pendingLimit = 50
	outbox := usecase.NewOutboxRelayUseCase(
		logger, botPort,
		notificationOutboxRepo,
		sched, pendingLimit,
	)
	if _, err := outbox.Start(ctx, 30*time.Second); err != nil {
		return nil, errors.Join(fmt.Errorf("can't start outbox relay: %w", err), sched.Shutdown())
	}

	poller := usecase.NewLinkPollingUseCase(
		logger, sched, linkStatePort,
		subscriptionRepo, int64(cfg.PollerIntervalMinutes),
	)
	if _, err := poller.Start(ctx, cfg.ConcurrencyBoundary); err != nil {
		return nil, errors.Join(fmt.Errorf("can't start polling service: %w", err), sched.Shutdown())
	}

	return sched, nil
}

func newBotPort(
	cfg infrastructure.Config, rcfg infrastructure.ReliabiltyConfig,
	logger *slog.Logger,
) (ports.BotPort, *grpc.ClientConn, *kafka.Writer, error) {
	if cfg.NotificationMethod == "kafka" {
		botPort, kafkaWriter, err := initKafkaClient(&cfg, logger)
		if err != nil {
			return nil, nil, nil, err
		}
		return botPort, nil, kafkaWriter, nil
	}

	grpcClient, err := grpc.NewClient(
		fmt.Sprintf("%v:%v", cfg.BotHost, cfg.BotGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		const msg = "can't create grpc client"
		if cfg.UseGRPC {
			return nil, nil, nil, fmt.Errorf("%s: %w", msg, err)
		}
		logger.Warn(msg, "err", err)
	}

	hc := http.Client{}
	client, err := botapi.NewClient(cfg.BotURL, botapi.WithHTTPClient(&hc))
	if err != nil {
		if grpcClient != nil {
			_ = grpcClient.Close()
		}
		return nil, nil, nil, fmt.Errorf("can't init bot http client: %w", err)
	}

	var grpcBotClient botpb.BotClient
	if grpcClient != nil {
		grpcBotClient = botpb.NewBotClient(grpcClient)
	}

	return adapters.NewBotAdapter(
		logger, client, grpcBotClient, cfg.UseGRPC, rcfg.ExternalServicesTimeoutSecond,
	), grpcClient, nil, nil
}

func newScrapperHTTPServer(
	cfg infrastructure.Config, logger *slog.Logger, scrapperPort ports.ScrapperPort,
) *http.Server {
	var httpServInterface scrapperapi.ServerInterface = server.NewScrapperServer(logger, scrapperPort)
	helpServ, err := newScrapperHelpServer(cfg)
	if err != nil {
		logger.Warn("can't init help server")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/helpers/seed-db", helpServ.SeedDBWithTestData)
	mux.HandleFunc("/helpers/teardown-db", helpServ.TearDownDB)
	handler := scrapperapi.HandlerFromMux(httpServInterface, mux)
	return &http.Server{
		Addr:    cfg.ServerURL,
		Handler: handler,
	}
}

func newScrapperHelpServer(cfg infrastructure.Config) (helpserv.HelpServ, error) {
	poolcfg, err := pgxpool.ParseConfig(cfg.DBConnString)
	if err != nil {
		return helpserv.HelpServ{}, err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolcfg)
	if err != nil {
		return helpserv.HelpServ{}, err
	}
	return helpserv.NewHelpServ(sql.NewDBSeeder(pool)), nil
}

func (app *scrapperRuntime) Run(ctx context.Context) error {
	app.scheduler.Start()

	errCh := make(chan error, 2)

	go func() {
		app.logger.Info("server has been started on", "addr", app.httpServer.Addr)
		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server exit with error: %w", err)
		}
	}()

	go func() {
		app.logger.Info("grpc server has been started on", "port", app.grpcPort)
		if err := app.grpcServer.Serve(app.listener); err != nil {
			errCh <- fmt.Errorf("grpc server exit with error: %w", err)
		}
		app.logger.Info("grpc server has been stopped")
	}()

	select {
	case <-ctx.Done():
		app.logger.Info("scrapper was shut down")
		return nil
	case err := <-errCh:
		return err
	}
}

func (app *scrapperRuntime) Close(ctx context.Context) error {
	var err error
	if app.httpServer != nil {
		err = joinRuntimeErr(err, app.httpServer.Shutdown(ctx))
	}
	if app.scheduler != nil {
		err = joinRuntimeErr(err, app.scheduler.Shutdown())
	}
	if app.grpcServer != nil {
		app.grpcServer.GracefulStop()
	}
	if app.listener != nil {
		err = joinRuntimeErr(err, app.listener.Close())
	}
	if app.grpcClient != nil {
		err = joinRuntimeErr(err, app.grpcClient.Close())
	}
	if app.kafkaWriter != nil {
		err = joinRuntimeErr(err, app.kafkaWriter.Close())
	}
	if app.closeStorage != nil {
		app.closeStorage()
	}
	if app.closeCache != nil {
		app.closeCache()
	}
	return err
}

func joinRuntimeErr(base error, next error) error {
	switch {
	case next == nil:
		return base
	case errors.Is(next, http.ErrServerClosed):
		return base
	case errors.Is(next, net.ErrClosed):
		return base
	default:
		return errors.Join(base, next)
	}
}

func newRepositories(ctx context.Context, cfg infrastructure.Config) (repositories, error) {
	switch cfg.DBAccessType {
	case "SQL":
		poolConf, err := pgxpool.ParseConfig(cfg.DBConnString)
		if err != nil {
			return repositories{}, err
		}
		pool, err := pgxpool.NewWithConfig(ctx, poolConf)
		if err != nil {
			return repositories{}, err
		}
		return repositories{
			chat:         sql.NewChatRepository(pool),
			subscription: sql.NewSubscriptionRepository(pool, cfg.DBLinksPageSize),
			outbox:       sql.NewNotificationOutboxRepository(pool),
			close:        pool.Close,
		}, nil
	case "ORM":
		poolConf, err := pgxpool.ParseConfig(cfg.DBConnString)
		if err != nil {
			return repositories{}, err
		}
		pool, err := pgxpool.NewWithConfig(ctx, poolConf)
		if err != nil {
			return repositories{}, err
		}
		return repositories{
			chat:         orm.NewChatRepository(pool),
			subscription: orm.NewSubscriptionRepository(pool, cfg.DBLinksPageSize),
			outbox:       orm.NewNotificationOutboxRepository(pool),
			close:        pool.Close,
		}, nil
	case "MEM":
		st := storage.New()
		return repositories{
			chat:         st,
			subscription: st,
			outbox:       st,
		}, nil
	default:
		return repositories{}, errors.New("unreachable")
	}
}

func initKafkaClient(cfg *infrastructure.Config, logger *slog.Logger) (ports.BotPort, *kafka.Writer, error) {
	brokers, err := cfg.KafkaBrokers()
	if err != nil {
		logger.Error("can't get kafka brokers", "err", err)
		return nil, nil, err
	}
	kw := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        cfg.KafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Logger:       kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger:  kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
	}
	var codec *avro.Codec
	for range 5 {
		codec, err = avro.NewCodec(
			cfg.SchemaRegistryURL, cfg.KafkaTopic, "/link_update.avsc",
		)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		logger.Error("can't init avro codec", "err", err)
		return nil, nil, err
	}
	botPort := adapters.NewBotKafkaAdapter(logger, kw, codec)
	return botPort, kw, nil
}
