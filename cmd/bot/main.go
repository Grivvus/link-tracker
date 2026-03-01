package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
)

func main() {
	cfg, err := infrastructure.LoadBotConfig()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	botapp, err := bot.New(ctx, cfg, logger)
	if err != nil {
		slog.Error("can't start bot", "with err", err)
		os.Exit(1)
	}

	slog.Info("bot has been started")
	botapp.Bot.Start(ctx)
}
