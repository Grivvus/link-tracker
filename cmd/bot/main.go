package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		slog.Error("Can't load .env file", "with err", err)
		os.Exit(1)
	}

	token, present := os.LookupEnv("BOT_TOKEN")
	if !present {
		slog.Error("Can't find BOT_TOKEN env variable", "with err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		slog.Error("Can't start bot", "with err", err)
		os.Exit(1)
	}

	b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "start", Description: "entry point to bot interaction"},
			{Command: "help", Description: "show help text"},
		},
	})

	b.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, handlerStart)
	b.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommand, handlerHelp)

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Неизвестная команда",
	})
	handlerHelp(ctx, b, update)
}

func handlerStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Привет! Это Бот link-tracker, для получения информации о всех командах, используйте /help",
	})
}

func handlerHelp(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "help command: TODO",
	})
}
