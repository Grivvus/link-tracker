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

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	botpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/server"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
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
	botPort := adapters.NewBotAdapter(logger, client, grpcBotClient, cfg.UseGRPC)
	_ = botPort // needed, when i implement notifier

	chatRepo := storage.NewChatStorage()
	subscriptionRepo := storage.NewSubscriptionStorage()
	scrapperPort := adapters.NewScrapperAdapter(logger, chatRepo, subscriptionRepo)
	var httpServInterface scrapper.ServerInterface = server.NewScrapperServer(logger, scrapperPort)
	mux := http.NewServeMux()
	h := scrapper.HandlerFromMux(httpServInterface, mux)
	httpServer := http.Server{
		Addr:    cfg.ServerURL,
		Handler: h,
	}

	pb.RegisterScrapperServer(grpcServer, server.NewScrapperGRPCServe(logger, scrapperPort))

	linkStatePort := adapters.NewLinkStateAdapter(logger)

	sched, err := gocron.NewScheduler()
	if err != nil {
		logger.Error("can't init scheduler", "err", err)
		exitCode = 1
		return
	}
	poller := usecase.NewLinkPollingUseCase(
		logger, sched, linkStatePort, botPort, subscriptionRepo, chatRepo,
	)
	poller.Start(ctx)

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
	logger.Info("scrapper was shutted down")
}
