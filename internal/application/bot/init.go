package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
)

func New(ctx context.Context, cfg infrastructure.BotConfig) (*bot.Bot, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(handlers.DefaultHandler),
	}
	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return nil, err
	}
	botInit(ctx, b)
	return b, nil
}
func botInit(ctx context.Context, b *bot.Bot) {
	b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "start", Description: "entry point to bot interaction"},
			{Command: "help", Description: "show help text"},
		},
	})

	b.RegisterHandler(
		bot.HandlerTypeMessageText, "start",
		bot.MatchTypeCommand, handlers.StartHandler,
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText, "help",
		bot.MatchTypeCommand, handlers.HelpHandler,
	)
}
