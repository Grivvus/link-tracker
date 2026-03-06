package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
)

type BotAdapter struct {
	bot *bot.Bot
	log *slog.Logger
}

func NewBotPort(bot *bot.Bot, logger *slog.Logger) ports.BotPort {
	return &BotAdapter{
		bot: bot,
		log: logger,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, in dto.LinkUpdate,
) error {
	for _, chatID := range in.TGChatIDs {
		go func(chatID int64) {
			_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   formNotificationMessage(in),
			})
			if err != nil {
				slog.Error(
					"can't send notification message",
					"err", err,
					"to chat", chatID,
				)
			}
		}(chatID)
	}
	return nil
}

func formNotificationMessage(updateInfo dto.LinkUpdate) string {
	const baseMsg = "One of the links you're following has been updated, check out what's new."
	return fmt.Sprintf(
		"%s\r\nurl: %s\r\nshort description: %s",
		baseMsg, updateInfo.URL, updateInfo.Description,
	)
}
