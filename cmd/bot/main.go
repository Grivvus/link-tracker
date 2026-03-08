package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/adapters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/server"
	oapibot "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
)

func main() {
	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{AddSource: true},
	))

	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	hc := http.Client{}
	c, err := scrapper.NewClient(fmt.Sprintf("http://%s/", cfg.ScrapperURL), scrapper.WithHTTPClient(&hc))
	if err != nil {
		logger.Error("can't create scrapper http client with", "err", err)
		os.Exit(1)
	}
	sp := adapters.NewScrapperPort(c, logger)

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

	go func() {
		logger.Info("server has been started on", "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exit with error", "err", err)
		}
	}()

	go func() {
		logger.Info("bot has been started")
		botapp.Bot.Start(ctx)
	}()
	<-ctx.Done()
	slog.Info("Catching signal, shut down application")
}
