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

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/adapters"
	notifications "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/server"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	scrapperpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	oapibot "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	scrapperapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	groupID         = "bot-events-reader"
	shutdownTimeout = 5 * time.Second
)

type botRuntime struct {
	logger      *slog.Logger
	httpServer  *http.Server
	grpcServer  *grpc.Server
	listener    net.Listener
	grpcPort    int
	botApp      *botapp.BotApplication
	grpcClient  *grpc.ClientConn
	kafkaReader *kafka.Reader
	kafkaWriter *kafka.Writer
}

func main() {
	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))

	if err := run(logger); err != nil {
		logger.Error("bot stopped with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) (err error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	app, err := newBotRuntime(ctx, cfg, logger)
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

func newBotRuntime(
	ctx context.Context, cfg infrastructure.Config, logger *slog.Logger,
) (app *botRuntime, err error) {
	app = &botRuntime{
		logger:     logger,
		grpcPort:   cfg.BotGRPCPort,
		grpcServer: grpc.NewServer(),
	}
	defer func() {
		if err == nil {
			return
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if app != nil {
			err = errors.Join(err, app.Close(shutdownCtx))
		}
	}()

	app.listener, err = newTCPListener(ctx, cfg.BotGRPCPort)
	if err != nil {
		return nil, fmt.Errorf("can't start tcp listener: %w", err)
	}

	scrapperPort, grpcClient, err := newScrapperPort(cfg, logger)
	if err != nil {
		return nil, err
	}
	app.grpcClient = grpcClient

	app.botApp, err = botapp.New(ctx, cfg, logger, scrapperPort)
	if err != nil {
		return nil, fmt.Errorf("can't start bot: %w", err)
	}

	botPort := adapters.NewBotPort(app.botApp.Bot, logger)
	app.httpServer = newBotHTTPServer(cfg, logger, botPort)
	pb.RegisterBotServer(app.grpcServer, server.NewBotGRPCServer(logger, botPort))

	if cfg.NotificationMethod == "kafka" {
		app.kafkaReader, app.kafkaWriter, err = initKafka(ctx, &cfg, logger, botPort)
		if err != nil {
			return nil, err
		}
	}

	return app, nil
}

func newTCPListener(ctx context.Context, port int) (net.Listener, error) {
	var lisConf net.ListenConfig
	return lisConf.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
}

func newScrapperPort(
	cfg infrastructure.Config, logger *slog.Logger,
) (ports.ScrapperPort, *grpc.ClientConn, error) {
	grpcClient, err := grpc.NewClient(
		fmt.Sprintf("%v:%v", cfg.ScrapperHost, cfg.ScrapperGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		const msg = "can't create grpc client"
		if cfg.UseGRPC {
			return nil, nil, fmt.Errorf("%s: %w", msg, err)
		}
		logger.Warn(msg, "err", err)
	}

	hc := http.Client{}
	client, err := scrapperapi.NewClient(
		fmt.Sprintf("http://%s/", cfg.ScrapperURL),
		scrapperapi.WithHTTPClient(&hc),
	)
	if err != nil {
		if grpcClient != nil {
			_ = grpcClient.Close()
		}
		return nil, nil, fmt.Errorf("can't create scrapper http client: %w", err)
	}

	var grpcScrapperClient scrapperpb.ScrapperClient
	if grpcClient != nil {
		grpcScrapperClient = scrapperpb.NewScrapperClient(grpcClient)
	}

	return adapters.NewScrapperPort(logger, client, grpcScrapperClient, cfg.UseGRPC), grpcClient, nil
}

func newBotHTTPServer(
	cfg infrastructure.Config, logger *slog.Logger, botPort ports.BotPort,
) *http.Server {
	var serv oapibot.ServerInterface = server.New(logger, botPort)
	mux := http.NewServeMux()
	handler := oapibot.HandlerFromMux(serv, mux)
	return &http.Server{
		Addr:    cfg.ServerURL,
		Handler: handler,
	}
}

func (app *botRuntime) Run(ctx context.Context) error {
	errCh := make(chan error, 2)

	go func() {
		app.logger.Info("server has been started on", "addr", app.httpServer.Addr)
		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server exit with error: %w", err)
		}
	}()

	go func() {
		app.logger.Info("grpc server has been started", "port", app.grpcPort)
		if err := app.grpcServer.Serve(app.listener); err != nil {
			errCh <- fmt.Errorf("grpc server exit with error: %w", err)
		}
		app.logger.Info("grpc server has been stopped")
	}()

	go func() {
		app.logger.Info("bot has been started")
		app.botApp.Bot.Start(ctx)
	}()

	select {
	case <-ctx.Done():
		app.logger.Info("catching signal, shut down application")
		return nil
	case err := <-errCh:
		return err
	}
}

func (app *botRuntime) Close(ctx context.Context) error {
	var err error
	if app.httpServer != nil {
		err = joinRuntimeErr(err, app.httpServer.Shutdown(ctx))
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
	if app.kafkaReader != nil {
		err = joinRuntimeErr(err, app.kafkaReader.Close())
	}
	if app.kafkaWriter != nil {
		err = joinRuntimeErr(err, app.kafkaWriter.Close())
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

func initKafka(
	ctx context.Context, cfg *infrastructure.Config,
	logger *slog.Logger, bp ports.BotPort,
) (*kafka.Reader, *kafka.Writer, error) {
	brokers, err := cfg.KafkaBrokers()
	if err != nil {
		logger.Error("invalid kafka brokers", "err", err)
		return nil, nil, err
	}
	dlqWriter := &kafka.Writer{
		Addr:        kafka.TCP(brokers...),
		Topic:       cfg.KafkaDLQTopic,
		Balancer:    &kafka.LeastBytes{},
		Logger:      kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger: kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
	}
	notificationsReader := kafka.NewReader(kafka.ReaderConfig{
		Topic:       cfg.KafkaTopic,
		Brokers:     brokers,
		Logger:      kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger: kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
		GroupID:     groupID,
	})
	codec, err := avro.NewCodec(
		cfg.SchemaRegistryURL, cfg.KafkaTopic, "/link_update.avsc",
	)
	if err != nil {
		logger.Error("can't create avro codec", "err", err)
		return nil, nil, err
	}
	reader := notifications.NewEventsReader(
		logger, bp, notificationsReader, dlqWriter, codec, cfg.KafkaMaxAttempts,
	)
	reader.Start(ctx)
	return notificationsReader, dlqWriter, nil
}
