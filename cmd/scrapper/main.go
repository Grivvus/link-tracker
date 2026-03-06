package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
	_ = client

	chatRepo := repository.NewChatRepository()
	trackingRepo := repository.NewTrackingRepository()
	scrapperPort := adapters.NewScrapperAdapter(logger, chatRepo, trackingRepo)
	var server scrapper.ServerInterface = server.NewScrapperServer(logger, scrapperPort)
	mux := http.NewServeMux()
	h := scrapper.HandlerFromMux(server, mux)
	httpServer := http.Server{
		Addr:    cfg.ScrapperURL,
		Handler: h,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server exit with error", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("scrapper was shutted down")
}
