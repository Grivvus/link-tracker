package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

type BotKafkaAdapter struct {
	logger      *slog.Logger
	kafkaWriter *kafka.Writer
}

func NewBotKafkaAdapter(logger *slog.Logger, kw *kafka.Writer) ports.BotPort {
	return &BotKafkaAdapter{
		logger:      logger,
		kafkaWriter: kw,
	}
}

func (b *BotKafkaAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	return b.kafkaNotify(ctx, event)
}

func (b *BotKafkaAdapter) kafkaNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	var body = bot.LinkUpdate{
		Id:           event.LinkID,
		Description:  event.Description,
		TgChatIds:    event.ChatIDs,
		Url:          event.RawURL,
		Title:        event.UpdateName,
		UpdateAuthor: event.UpdateAuthor,
		UpdatedAt:    event.UpdatedAt.Format(time.RFC3339),
	}
	key, err := json.Marshal(body.Id)
	if err != nil {
		return fmt.Errorf("%w: can't encode message", ports.ErrBadParams)
	}
	value, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%w: can't encode message", ports.ErrBadParams)
	}

	err = b.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return fmt.Errorf(
			"%w can't send message caused by: %w",
			ports.ErrBadResponse, err,
		)
	}
	return nil
}
