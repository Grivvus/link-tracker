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

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/adapters"
	notifications "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/server"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	scrapperpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	oapibot "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const groupID = "bot-events-reader"

func main() {
	var exitCode = 0
	// call os.Exit in defer, so all others defer blocks could be called
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
		logger.Error(err.Error())
		exitCode = 1
		return
	}

	var lisConf net.ListenConfig
	lis, err := lisConf.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.BotGRPCPort))
	if err != nil {
		logger.Error("can't start tcp listener", "err", err)
		exitCode = 1
		return
	}
	defer func() { _ = lis.Close() }()

	grpcServ := grpc.NewServer()
	defer grpcServ.GracefulStop()

	grpcClient, err := grpc.NewClient(
		fmt.Sprintf("%v:%v", cfg.ScrapperHost, cfg.ScrapperGRPCPort),
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
	grpcScrapperClient := scrapperpb.NewScrapperClient(grpcClient)

	hc := http.Client{}
	c, err := scrapper.NewClient(fmt.Sprintf("http://%s/", cfg.ScrapperURL), scrapper.WithHTTPClient(&hc))
	if err != nil {
		logger.Error("can't create scrapper http client with", "err", err)
		exitCode = 1
		return
	}
	sp := adapters.NewScrapperPort(logger, c, grpcScrapperClient, cfg.UseGRPC)

	botapp, err := bot.New(ctx, cfg, logger, sp)
	if err != nil {
		slog.Error("can't start bot", "with err", err)
		exitCode = 1
		return
	}

	bp := adapters.NewBotPort(botapp.Bot, logger)

	var serv oapibot.ServerInterface = server.New(logger, bp)
	mux := http.NewServeMux()
	h := oapibot.HandlerFromMux(serv, mux)
	s := http.Server{
		Addr:    cfg.ServerURL,
		Handler: h,
	}

	pb.RegisterBotServer(grpcServ, server.NewBotGRPCServer(logger, bp))

	if cfg.NotificationMethod == "kafka" {
		brokers, err := cfg.KafkaBrokers()
		if err != nil {
			logger.Error("invalid kafka brokers", "err", err)
			exitCode = 1
			return
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
		defer func() { _ = dlqWriter.Close() }()
		defer func() { _ = notificationsReader.Close() }()
		reader := notifications.NewEventsReader(
			logger, bp, notificationsReader, dlqWriter, cfg.KafkaMaxAttempts,
		)
		reader.Start(ctx)
	}

	go func() {
		logger.Info("server has been started on", "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exit with error", "err", err)
		}
	}()

	go func() {
		logger.Info("grpc server has been started", "port", cfg.BotGRPCPort)
		if err := grpcServ.Serve(lis); err != nil {
			logger.Error("grpc server exit with error", "err", err)
		}
		logger.Info("grpc server has been stopped")
	}()

	go func() {
		logger.Info("bot has been started")
		botapp.Bot.Start(ctx)
	}()
	<-ctx.Done()
	slog.Info("Catching signal, shut down application")
}
