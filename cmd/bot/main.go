package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/server"
	oapibot "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

func main() {
	cfg, err := infrastructure.LoadBotConfig()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	botapp, err := bot.New(ctx, cfg, logger)
	if err != nil {
		slog.Error("can't start bot", "with err", err)
		os.Exit(1)
	}
	var serv oapibot.ServerInterface = server.New(logger)
	mux := http.NewServeMux()
	h := oapibot.HandlerFromMux(serv, mux)
	s := http.Server{
		Addr:    fmt.Sprintf("%v:%v", cfg.Host, cfg.Port),
		Handler: h,
	}

	go func() {
		logger.Info("server has been started on", "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
