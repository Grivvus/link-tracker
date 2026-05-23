package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
)

var ErrRetryPolicyExceeded = errors.New("retry policy exceeded")
var ErrBadSerialization = errors.New("failed to decode update")

type EventsHandler interface {
	Handle(context.Context, model.LinkUpdatedEvent) error
}

type eventsHandlerBot struct {
	bp ports.BotPort
}

func (e eventsHandlerBot) Handle(ctx context.Context, event model.LinkUpdatedEvent) error {
	return e.bp.NotifyAboutUpdatedLinks(ctx, event)
}

func HandlerFromBotPort(bp ports.BotPort) EventsHandler {
	return eventsHandlerBot{bp: bp}
}

type eventsHandlerAgent struct {
	processor usecase.EventUpdateProcessorUseCase
}

func (e eventsHandlerAgent) Handle(ctx context.Context, event model.LinkUpdatedEvent) error {
	return e.processor.Process(ctx, ports.ReceivedEvent{
		Event: event,
		MarkProcessed: func(context.Context) error {
			return nil
		},
		MarkFailed: func(context.Context, error) error {
			return nil
		},
	})
}

func HandlerFromEventsProcessor(processor usecase.EventUpdateProcessorUseCase) EventsHandler {
	return eventsHandlerAgent{processor: processor}
}

type EventsReader struct {
	logger       *slog.Logger
	handler      EventsHandler
	eventsReader *kafka.Reader
	dlqWriter    *kafka.Writer
	avroCodec    *avro.Codec
	retryPolicy  int
}

func NewEventsReader(
	logger *slog.Logger, handler EventsHandler,
	eventsReader *kafka.Reader, dlqWriter *kafka.Writer,
	codec *avro.Codec, retryPolicy int,
) *EventsReader {
	return &EventsReader{
		logger:       logger,
		eventsReader: eventsReader,
		handler:      handler,
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
		err = r.handler.Handle(ctx, update)
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
