package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
)

var ErrRetryPolicyExceeded = fmt.Errorf("retry policy exceeded")
var ErrBadSerialization = fmt.Errorf("failed to decode update")

type EventsReader struct {
	logger       *slog.Logger
	port         ports.BotPort
	eventsReader *kafka.Reader
	dlqWriter    *kafka.Writer
	avroCodec    *avro.Codec
	retryPolicy  int
}

func NewEventsReader(
	logger *slog.Logger, port ports.BotPort,
	eventsReader *kafka.Reader, dlqWriter *kafka.Writer,
	codec *avro.Codec, retryPolicy int,
) *EventsReader {
	return &EventsReader{
		logger:       logger,
		port:         port,
		eventsReader: eventsReader,
		dlqWriter:    dlqWriter,
		retryPolicy:  retryPolicy,
		avroCodec:    codec,
	}
}

func (r *EventsReader) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := r.eventsReader.FetchMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					r.logger.Error("ReadMessage error", "err", err)
					continue
				}
				ok, err := r.handleMessage(ctx, msg)
				if err != nil {
					r.logger.Error("Handle Message error", "err", err)
				}
				if !ok {
					continue
				}
				if err := r.eventsReader.CommitMessages(ctx, msg); err != nil {
					r.logger.Error("Commit Message error", "err", err)
				}
			}
		}
	}()
}

func (r *EventsReader) handleMessage(ctx context.Context, msg kafka.Message) (bool, error) {
	update, err := r.avroCodec.Decode(msg.Value)
	if err != nil {
		return r.writeDLQ(ctx, msg, fmt.Errorf("%w: %w", ErrBadSerialization, err))
	}
	for range r.retryPolicy {
		err = r.port.NotifyAboutUpdatedLinks(ctx, update)
		if err == nil {
			r.logger.Info("message has been sent", "msg", update)
			return true, nil
		}
	}
	return r.writeDLQ(ctx, msg, ErrRetryPolicyExceeded)
}

func (r *EventsReader) writeDLQ(
	ctx context.Context, msg kafka.Message, cause error,
) (bool, error) {
	msg.Topic = ""
	r.logger.Error("can't handle message", "msg", msg, "err", cause)
	err := r.dlqWriter.WriteMessages(ctx, msg)
	if err != nil {
		return false, err
	}
	return true, nil
}
