package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
)

type BotKafkaAdapter struct {
	logger      *slog.Logger
	kafkaWriter *kafka.Writer
	avroCodec   *avro.Codec
}

func NewBotKafkaAdapter(
	logger *slog.Logger, kw *kafka.Writer, codec *avro.Codec,
) ports.BotPort {
	return &BotKafkaAdapter{
		logger:      logger,
		kafkaWriter: kw,
		avroCodec:   codec,
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
	rawKey := strconv.Itoa(int(event.LinkID)) + event.UpdateName
	key, err := json.Marshal(rawKey)
	if err != nil {
		return fmt.Errorf("%w: can't encode message", ports.ErrBadParams)
	}
	value, err := b.avroCodec.Encode(event)
	if err != nil {
		return fmt.Errorf("%w: can't encode message - %w", ports.ErrBadParams, err)
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
