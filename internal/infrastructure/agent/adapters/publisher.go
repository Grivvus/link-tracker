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

type ProcessedUpdatePublisherAdapter struct {
	logger *slog.Logger
	writer *kafka.Writer
	codec  *avro.Codec
}

func NewProcessedUpdatePublisher(
	logger *slog.Logger, kafkaWriter *kafka.Writer,
	codec *avro.Codec,
) ports.ProcessedUpdatePublisher {
	return &ProcessedUpdatePublisherAdapter{
		logger: logger,
		writer: kafkaWriter,
		codec:  codec,
	}
}

func (p *ProcessedUpdatePublisherAdapter) PublishProcessedEvent(
	ctx context.Context, processedEvent model.LinkUpdatedEvent,
) error {
	rawKey := strconv.Itoa(int(processedEvent.LinkID)) + processedEvent.UpdateName
	key, err := json.Marshal(rawKey)
	if err != nil {
		return fmt.Errorf("%w: can't encode message", ports.ErrBadParams)
	}
	value, err := p.codec.Encode(processedEvent)
	if err != nil {
		return fmt.Errorf("%w: can't encode message - %w", ports.ErrBadParams, err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
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
