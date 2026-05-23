package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	kafkago "github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
)

type OffsetTracker interface {
	MarkProcessed(context.Context, kafkago.Message) error
	MarkFailed(context.Context, kafkago.Message, error) error
}

type DeferredEventsReader struct {
	logger        *slog.Logger
	eventsReader  *kafkago.Reader
	dlqWriter     *kafkago.Writer
	avroCodec     *avro.Codec
	offsetTracker OffsetTracker
	retryPolicy   int
}

func NewDeferredEventsReader(
	logger *slog.Logger,
	eventsReader *kafkago.Reader,
	dlqWriter *kafkago.Writer,
	codec *avro.Codec,
	offsetTracker OffsetTracker,
	retryPolicy int,
) *DeferredEventsReader {
	return &DeferredEventsReader{
		logger:        logger,
		eventsReader:  eventsReader,
		dlqWriter:     dlqWriter,
		avroCodec:     codec,
		offsetTracker: offsetTracker,
		retryPolicy:   retryPolicy,
	}
}

func (r *DeferredEventsReader) Start(ctx context.Context) (<-chan ports.ReceivedEvent, error) {
	ch := make(chan ports.ReceivedEvent)
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
				updateEvent, err := r.avroCodec.Decode(msg.Value)
				if err != nil {
					r.logger.Error("can't handle message", "msg", msg, "err", err)
					msg.Topic = "" // reset topic before write to dlq
					r.dlqWriter.WriteMessages(ctx, msg)
					continue
				}
				ch <- ports.ReceivedEvent{
					Event: updateEvent,
					MarkProcessed: func(ctx context.Context) error {
						return r.offsetTracker.MarkProcessed(ctx, msg)
					},
					MarkFailed: func(ctx context.Context, err error) error {
						return r.offsetTracker.MarkFailed(ctx, msg, err)
					},
				}
			}
		}
	}()

	return ch, nil
}

type PartitionOffsetTracker struct {
	mu                 sync.Mutex
	logger             *slog.Logger
	eventsReader       *kafkago.Reader
	dlqWriter          *kafkago.Writer
	codec              *avro.Codec
	lastOffsetCommited int64
	offsetProcessed    map[int64]kafkago.Message
}

func NewPartitionOffsetTracker(
	logger *slog.Logger,
	eventsReader *kafkago.Reader,
	dlqWriter *kafkago.Writer,
	codec *avro.Codec,
) OffsetTracker {
	return &PartitionOffsetTracker{
		logger:             logger,
		eventsReader:       eventsReader,
		codec:              codec,
		dlqWriter:          dlqWriter,
		lastOffsetCommited: -1,
		offsetProcessed:    make(map[int64]kafkago.Message),
	}
}

func (t *PartitionOffsetTracker) MarkProcessed(ctx context.Context, msg kafkago.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if msg.Offset <= t.lastOffsetCommited {
		t.logger.Warn("Message was commited before it was processed", "offset", msg.Offset)
		return nil
	}
	// nothing is commited yet
	if t.lastOffsetCommited == -1 {
		err := t.eventsReader.CommitMessages(ctx, msg)
		if err != nil {
			return fmt.Errorf("can't commit processed message: %w", err)
		}
		t.lastOffsetCommited = msg.Offset
		return nil
	}

	t.offsetProcessed[msg.Offset] = msg

	next := t.lastOffsetCommited + 1
	start := next
	for {
		_, ok := t.offsetProcessed[next]
		if !ok {
			break
		}
		next++
	}
	if start != next {
		err := t.eventsReader.CommitMessages(ctx, t.offsetProcessed[next-1])
		if err != nil {
			return fmt.Errorf("can't commit processed message: %w", err)
		}
		t.lastOffsetCommited = next - 1
		for idx := start; idx < next; idx++ {
			delete(t.offsetProcessed, idx)
		}
	}
	return nil
}

func (t *PartitionOffsetTracker) MarkFailed(ctx context.Context, msg kafkago.Message, err error) error {
	t.logger.Error("write msg to dlq", "failed cause", err)
	msg.Topic = ""
	err = t.dlqWriter.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("can't write to dlq")
	}
	return t.MarkProcessed(ctx, msg)
}
