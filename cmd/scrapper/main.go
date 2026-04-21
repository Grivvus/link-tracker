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
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
	botpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/clients"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/server"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/orm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var exitCode = 0
	defer func() {
		os.Exit(exitCode)
	}()

	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		logger.Error("can't load config", "err", err)
		exitCode = 1
		return
	}

	var lisConf net.ListenConfig
	lis, err := lisConf.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.ScrapperGRPCPort))
	if err != nil {
		logger.Error("can't create tcp listener", "err", err)
		exitCode = 1
		return
	}
	defer func() { _ = lis.Close() }()
	grpcServer := grpc.NewServer()
	defer grpcServer.GracefulStop()

	grpcClient, err := grpc.NewClient(
		fmt.Sprintf("%v:%v", cfg.BotHost, cfg.BotGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		const msg = "can't create grpc client"
		if cfg.UseGRPC {
			logger.Error(msg, "err", err)
			exitCode = 1
			return
		}
		logger.Warn(msg, "err", err)
	}
	defer func() {
		if grpcClient != nil {
			_ = grpcClient.Close()
		}
	}()
	grpcBotClient := botpb.NewBotClient(grpcClient)

	hc := http.Client{}
	client, err := bot.NewClient(cfg.BotURL, bot.WithHTTPClient(&hc))
	if err != nil {
		logger.Error("can't init bot http client", "err", err)
		exitCode = 1
		return
	}
	var botPort ports.BotPort
	if cfg.NotificationMethod == "kafka" {
		var kw *kafka.Writer
		botPort, kw, err = initKafkaClient(&cfg, logger)
		if err != nil {
			exitCode = 1
			return
		}
		defer func() { _ = kw.Close() }()
	} else {
		botPort = adapters.NewBotAdapter(logger, client, grpcBotClient, cfg.UseGRPC)
	}

	chatRepo, subscriptionRepo, notificationOutboxRepo, err := newRepositories(ctx, cfg)
	if err != nil {
		logger.Error("can't init repository", "err", err)
		exitCode = 1
		return
	}
	scrapperPort := adapters.NewScrapperAdapter(logger, chatRepo, subscriptionRepo)
	var httpServInterface scrapper.ServerInterface = server.NewScrapperServer(logger, scrapperPort)
	mux := http.NewServeMux()
	h := scrapper.HandlerFromMux(httpServInterface, mux)
	httpServer := http.Server{
		Addr:    cfg.ServerURL,
		Handler: h,
	}

	pb.RegisterScrapperServer(grpcServer, server.NewScrapperGRPCServe(logger, scrapperPort))

	linkStatePort := adapters.NewLinkStateAdapter(
		logger,
		clients.NewGitHubClient(logger),
		clients.NewStackoverflowClient(logger),
	)

	sched, err := gocron.NewScheduler()
	defer func() { _ = sched.Shutdown() }()
	if err != nil {
		logger.Error("can't init scheduler", "err", err)
		exitCode = 1
		return
	}
	outbox := usecase.NewOutboxRelayUseCase(
		logger, botPort,
		notificationOutboxRepo,
		sched, 50,
	)
	poller := usecase.NewLinkPollingUseCase(
		logger, sched, linkStatePort,
		subscriptionRepo, int64(cfg.PollerIntervalMinutes),
	)
	_, err = outbox.Start(ctx, 30*time.Second)
	_, err = poller.Start(ctx, cfg.ConcurrencyBoundary)

	sched.Start()
	if err != nil {
		logger.Error("can't start polling service", "err", err)
		exitCode = 1
		return
	}

	go func() {
		logger.Info("server has been started on", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exit with error", "err", err)
		}
	}()

	go func() {
		logger.Info("grpc server has been started on", "port", cfg.ScrapperGRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server exit with error", "err", err)
		}
		logger.Info("grpc server has been stopped")
	}()

	<-ctx.Done()
	logger.Info("scrapper was shut down")
}

func newRepositories(
	ctx context.Context, cfg infrastructure.Config,
) (repository.ChatRepository, repository.SubscriptionRepository, repository.NotificationOutboxRepository, error) {
	switch cfg.DBAccessType {
	case "SQL":
		poolConf, err := pgxpool.ParseConfig(cfg.DBConnString)
		if err != nil {
			return nil, nil, nil, err
		}
		pool, err := pgxpool.NewWithConfig(ctx, poolConf)
		if err != nil {
			return nil, nil, nil, err
		}
		chat := sql.NewChatRepository(pool)
		sub := sql.NewSubscriptionRepository(pool, cfg.DBLinksPageSize)
		outbox := sql.NewNotificationOutboxRepository(pool)
		return chat, sub, outbox, nil
	case "ORM":
		poolConf, err := pgxpool.ParseConfig(cfg.DBConnString)
		if err != nil {
			return nil, nil, nil, err
		}
		pool, err := pgxpool.NewWithConfig(ctx, poolConf)
		if err != nil {
			return nil, nil, nil, err
		}
		chat := orm.NewChatRepository(pool)
		sub := orm.NewSubscriptionRepository(pool, cfg.DBLinksPageSize)
		outbox := orm.NewNotificationOutboxRepository(pool)
		return chat, sub, outbox, nil
	case "MEM":
		st := storage.New()
		return st, st, st, nil
	default:
		return nil, nil, nil, errors.New("unreachable")
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
	codec, err := avro.NewCodec(
		cfg.SchemaRegistryURL, cfg.KafkaTopic, "/link_update.avsc",
	)
	if err != nil {
		logger.Error("can't init avro codec", "err", err)
		return nil, nil, err
	}
	botPort := adapters.NewBotKafkaAdapter(logger, kw, codec)
	return botPort, kw, nil
}
