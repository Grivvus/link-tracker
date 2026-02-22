package handlers

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	defaultMsgText = "Неизвестная команда"
	startMsgText   = "Привет! Это Бот link-tracker, для получения информации о всех командах, используйте /help"
	helpMsgText    = `
    start - Выводит приветственное сообщение,
    help - Выводит этот текст
	`
)

func DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Warn("Default handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   defaultMsgText,
	})
	HelpHandler(ctx, b, update)
}

func StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Info("Start handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   startMsgText,
	})
}

func HelpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Info("Help handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   helpMsgText,
	})
}
