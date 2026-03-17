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

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/server"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	scrapperpb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	oapibot "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.BotGRPCPort))
	if err != nil {
		logger.Error("can't start tcp listener", "err", err)
		os.Exit(1)
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
			os.Exit(1)
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
		os.Exit(1)
	}
	sp := adapters.NewScrapperPort(logger, c, grpcScrapperClient, cfg.UseGRPC)

	botapp, err := bot.New(ctx, cfg, logger, sp)
	if err != nil {
		slog.Error("can't start bot", "with err", err)
		os.Exit(1)
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
