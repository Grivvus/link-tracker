package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	appmetrics "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/metrics"
)

type BotAdapter struct {
	bot     *bot.Bot
	log     *slog.Logger
	metrics *appmetrics.BotMetrics
}

func NewBotPort(bot *bot.Bot, logger *slog.Logger, botmetrics *appmetrics.BotMetrics) ports.BotPort {
	return &BotAdapter{
		bot:     bot,
		log:     logger,
		metrics: botmetrics,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	for _, chatID := range event.ChatIDs {
		go func(chatID int64) {
			_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   formNotificationMessage(event),
			})
			if err != nil {
				slog.Error(
					"can't send notification message",
					"err", err,
					"to chat", chatID,
				)
				return
			}

			b.metrics.SentNotificationsTotal.Inc()
		}(chatID)
	}
	return nil
}

func formNotificationMessage(updateInfo model.LinkUpdatedEvent) string {
	const baseMsg = "One of the links you're following has been updated, check out what's new."
	return fmt.Sprintf(
		`
%s
priority: %s
url: %s
updated at: %v
title: %s
update author: %s
short description: %s
`,
		baseMsg, updateInfo.Priority.String(),
		updateInfo.RawURL, updateInfo.UpdatedAt,
		updateInfo.UpdateName, updateInfo.UpdateAuthor, updateInfo.Description,
	)
}
