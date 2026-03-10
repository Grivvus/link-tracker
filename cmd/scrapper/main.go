package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/server"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))
	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		logger.Error("can't load config", "err", err)
		os.Exit(1)
	}

	hc := http.Client{}
	client, err := bot.NewClient(cfg.BotURL, bot.WithHTTPClient(&hc))
	if err != nil {
		logger.Error("can't init bot http client", "err", err)
		os.Exit(1)
	}
	botPort := adapters.NewBotAdapter(logger, client)
	_ = botPort // needed, when i implement notifier

	chatRepo := storage.NewChatStorage()
	subscriptionRepo := storage.NewSubscriptionStorage()
	scrapperPort := adapters.NewScrapperAdapter(logger, chatRepo, trackingRepo)
	var server scrapper.ServerInterface = server.NewScrapperServer(logger, scrapperPort)
	mux := http.NewServeMux()
	h := scrapper.HandlerFromMux(server, mux)
	httpServer := http.Server{
		Addr:    cfg.ServerURL,
		Handler: h,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	linkStatePort := adapters.NewLinkStateAdapter(logger)

	sched, err := gocron.NewScheduler()
	if err != nil {
		logger.Error("can't init scheduler", "err", err)
		os.Exit(1)
	}
	poller := usecase.NewLinkPollingUseCase(logger, sched, linkStatePort, botPort, trackingRepo)
	poller.Start(ctx)

	go func() {
		logger.Info("server has been started on", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exit with error", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("scrapper was shutted down")
}
