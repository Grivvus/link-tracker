package usecase_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
)

type passthroughSummarizer struct{}

func (passthroughSummarizer) Summarize(
	_ context.Context, event model.LinkUpdatedEvent,
) (model.LinkUpdatedEvent, error) {
	return event, nil
}

type groupingPublisher struct {
	events chan model.LinkUpdatedEvent
}

func newGroupingPublisher(size int) *groupingPublisher {
	return &groupingPublisher{
		events: make(chan model.LinkUpdatedEvent, size),
	}
}

func (p *groupingPublisher) PublishProcessedEvent(
	_ context.Context, event model.LinkUpdatedEvent,
) error {
	p.events <- event
	return nil
}

func TestInMemoryUpdateGrouper_FlushPublishesMaxPriority(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const chatID int64 = 101
	publisher := newGroupingPublisher(1)
	grouper := newTestGrouper(ctx, time.Hour, publisher)

	grouper.Add(ctx, receivedGroupingEvent(groupingEvent(func(event *model.LinkUpdatedEvent) {
		event.Priority = model.PriorityLow
	})), chatID)
	grouper.Add(ctx, receivedGroupingEvent(groupingEvent(func(event *model.LinkUpdatedEvent) {
		event.Priority = model.PriorityHigh
	})), chatID)
	grouper.Add(ctx, receivedGroupingEvent(groupingEvent(func(event *model.LinkUpdatedEvent) {
		event.Priority = model.PriorityMedium
	})), chatID)

	require.NoError(t, grouper.Flush(ctx))
	published := requirePublishedEvent(t, publisher, time.Second)
	require.Equal(t, model.PriorityHigh, published.Priority)
}

func TestInMemoryUpdateGrouper_GroupsForConfiguredWindow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const chatID int64 = 101
	window := 300 * time.Millisecond
	publisher := newGroupingPublisher(1)
	grouper := newTestGrouper(ctx, window, publisher)

	startedAt := time.Now()
	grouper.Add(ctx, receivedGroupingEvent(groupingEvent()), chatID)
	published := requirePublishedEvent(t, publisher, 2*window)
	publishedAfter := time.Since(startedAt)

	require.Equal(t, []int64{chatID}, published.ChatIDs)
	require.GreaterOrEqual(t, publishedAfter, window)
	require.LessOrEqual(t, publishedAfter, 2*window)
}

func TestInMemoryUpdateGrouper_SingleEventGroupKeepsMainFields(t *testing.T) {
	t.Skip()
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const chatID int64 = 101
	publisher := newGroupingPublisher(1)
	grouper := newTestGrouper(ctx, time.Hour, publisher)
	event := groupingEvent(func(event *model.LinkUpdatedEvent) {
		event.ChatIDs = []int64{chatID}
		event.Priority = model.PriorityLow
	})

	grouper.Add(ctx, receivedGroupingEvent(event), chatID)

	require.NoError(t, grouper.Flush(ctx))
	published := requirePublishedEvent(t, publisher, time.Second)
	require.Equal(t, event, published)
}

func TestInMemoryUpdateGrouper_ConcurrentAddGroupsAllEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		chatCount     = 8
		eventsPerChat = 25
	)
	publisher := newGroupingPublisher(chatCount)
	grouper := newTestGrouper(ctx, time.Hour, publisher)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for chatIdx := range chatCount {
		chatID := int64(1000 + chatIdx)
		for eventIdx := range eventsPerChat {
			wg.Go(func() {
				<-start
				grouper.Add(ctx, receivedGroupingEvent(groupingEvent(func(event *model.LinkUpdatedEvent) {
					event.ChatIDs = []int64{chatID}
					event.Description = fmt.Sprintf("chat-%d-event-%d", chatID, eventIdx)
				})), chatID)
			})
		}
	}

	close(start)
	wg.Wait()

	require.NoError(t, grouper.Flush(ctx))

	publishedByChat := make(map[int64]model.LinkUpdatedEvent, chatCount)
	for range chatCount {
		published := requirePublishedEvent(t, publisher, time.Second)
		require.Len(t, published.ChatIDs, 1)
		chatID := published.ChatIDs[0]
		require.NotContains(t, publishedByChat, chatID)
		publishedByChat[chatID] = published
	}

	require.Len(t, publishedByChat, chatCount)
	require.Zero(t, len(publisher.events))
	for chatIdx := range chatCount {
		chatID := int64(1000 + chatIdx)
		published, ok := publishedByChat[chatID]
		require.Truef(t, ok, "missing grouped event for chat %d", chatID)
		require.Equal(t, eventsPerChat, strings.Count(published.Description, "\n"))
	}
}

func newTestGrouper(
	ctx context.Context,
	window time.Duration,
	publisher ports.ProcessedUpdatePublisher,
) *usecase.InMemoryUpdateGrouper {
	return usecase.NewInMemoryUpdateGrouper(
		ctx,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		window,
		usecase.NewDefaultUpdateMerger(),
		passthroughSummarizer{},
		publisher,
	)
}

func requirePublishedEvent(
	t *testing.T,
	publisher *groupingPublisher,
	timeout time.Duration,
) model.LinkUpdatedEvent {
	t.Helper()

	select {
	case event := <-publisher.events:
		return event
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for published grouped event")
		return model.LinkUpdatedEvent{}
	}
}

func receivedGroupingEvent(event model.LinkUpdatedEvent) ports.ReceivedEvent {
	return ports.ReceivedEvent{
		Event: event,
		MarkProcessed: func(context.Context) error {
			return nil
		},
		MarkFailed: func(context.Context, error) error {
			return nil
		},
	}
}

func groupingEvent(opts ...func(*model.LinkUpdatedEvent)) model.LinkUpdatedEvent {
	event := model.LinkUpdatedEvent{
		LinkID:       1,
		RawURL:       "https://github.com/example/repo",
		UpdatedAt:    time.Date(2026, time.May, 23, 12, 0, 0, 0, time.UTC),
		UpdateName:   "Issue title",
		UpdateAuthor: "allowed-author",
		Description:  "valid update description",
		Priority:     model.PriorityMedium,
		ChatIDs:      []int64{101},
	}
	for _, opt := range opts {
		opt(&event)
	}
	return event
}
