package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type fakeRelayBotPort struct {
	attempts []model.LinkUpdatedEvent
	failures map[string]error
}

func (f *fakeRelayBotPort) NotifyAboutUpdatedLinks(
	_ context.Context, event model.LinkUpdatedEvent,
) error {
	f.attempts = append(f.attempts, cloneRelayEvent(event))
	if err, ok := f.failures[event.UpdateName]; ok {
		return err
	}
	return nil
}

type getPendingCall struct {
	startsWith int64
	limit      int32
}

type markFailedCall struct {
	id     int64
	reason string
}

type pendingResponse struct {
	events []model.OutboxLinkUpdateEvent
	err    error
}

type fakeRelayOutboxRepo struct {
	responses       map[int64]pendingResponse
	markSentErr     error
	markFailedErrs  map[int64]error
	getPendingCalls []getPendingCall
	markSentCalls   [][]int64
	markFailedCalls []markFailedCall
}

func (f *fakeRelayOutboxRepo) GetPending(
	_ context.Context, startsWith int64, limit int32,
) ([]model.OutboxLinkUpdateEvent, error) {
	f.getPendingCalls = append(f.getPendingCalls, getPendingCall{
		startsWith: startsWith,
		limit:      limit,
	})
	resp, ok := f.responses[startsWith]
	if !ok {
		return []model.OutboxLinkUpdateEvent{}, nil
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return cloneOutboxEvents(resp.events), nil
}

func (f *fakeRelayOutboxRepo) MarkSent(_ context.Context, ids []int64) error {
	f.markSentCalls = append(f.markSentCalls, append([]int64(nil), ids...))
	if f.markSentErr != nil {
		return f.markSentErr
	}
	return nil
}

func (f *fakeRelayOutboxRepo) MarkFailed(_ context.Context, id int64, reason string) error {
	f.markFailedCalls = append(f.markFailedCalls, markFailedCall{
		id:     id,
		reason: reason,
	})
	if err, ok := f.markFailedErrs[id]; ok {
		return err
	}
	return nil
}

func testOutboxRelayLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOutboxRelayUseCase_Start_CreatesJob(t *testing.T) {
	t.Parallel()

	sched, err := gocron.NewScheduler()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, sched.Shutdown())
	}()

	uc := NewOutboxRelayUseCase(
		testOutboxRelayLogger(),
		&fakeRelayBotPort{},
		&fakeRelayOutboxRepo{},
		sched,
		2,
	)

	job, err := uc.Start(context.Background(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, job)
}

func TestOutboxRelayUseCase_WorkerFn_SendsPendingEventsAcrossPages(t *testing.T) {
	t.Parallel()

	events := []model.OutboxLinkUpdateEvent{
		makeOutboxEvent(1, "issue-1"),
		makeOutboxEvent(2, "issue-2"),
		makeOutboxEvent(3, "issue-3"),
	}
	repo := &fakeRelayOutboxRepo{
		responses: map[int64]pendingResponse{
			0: {events: events[:2]},
			2: {events: events[2:]},
		},
	}
	bot := &fakeRelayBotPort{}
	uc := OutboxRelayUseCase{
		logger:     testOutboxRelayLogger(),
		sender:     bot,
		outboxRepo: repo,
		limit:      2,
	}

	err := uc.workerFn(context.Background())

	require.NoError(t, err)
	require.Equal(t, []getPendingCall{
		{startsWith: 0, limit: 2},
		{startsWith: 2, limit: 2},
	}, repo.getPendingCalls)
	require.Equal(t, [][]int64{{1, 2}, {3}}, repo.markSentCalls)
	require.Empty(t, repo.markFailedCalls)
	require.Equal(t, outboxEventsToRelayEvents(events), bot.attempts)
}

func TestOutboxRelayUseCase_WorkerFn_MarksFailedAndContinues(t *testing.T) {
	t.Parallel()

	events := []model.OutboxLinkUpdateEvent{
		makeOutboxEvent(1, "issue-1"),
		makeOutboxEvent(2, "issue-2"),
		makeOutboxEvent(3, "issue-3"),
	}
	repo := &fakeRelayOutboxRepo{
		responses: map[int64]pendingResponse{
			0: {events: events[:2]},
			2: {events: events[2:]},
		},
	}
	bot := &fakeRelayBotPort{
		failures: map[string]error{
			"issue-2": errors.New("bot is down"),
		},
	}
	uc := OutboxRelayUseCase{
		logger:     testOutboxRelayLogger(),
		sender:     bot,
		outboxRepo: repo,
		limit:      2,
	}

	err := uc.workerFn(context.Background())

	require.NoError(t, err)
	require.Equal(t, [][]int64{{1}, {3}}, repo.markSentCalls)
	require.Equal(t, []markFailedCall{{
		id:     2,
		reason: "bot is down",
	}}, repo.markFailedCalls)
	require.Equal(t, outboxEventsToRelayEvents(events), bot.attempts)
}

func TestOutboxRelayUseCase_WorkerFn_ReturnsGetPendingError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db unavailable")
	repo := &fakeRelayOutboxRepo{
		responses: map[int64]pendingResponse{
			0: {err: wantErr},
		},
	}
	bot := &fakeRelayBotPort{}
	uc := OutboxRelayUseCase{
		logger:     testOutboxRelayLogger(),
		sender:     bot,
		outboxRepo: repo,
		limit:      2,
	}

	err := uc.workerFn(context.Background())

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, []getPendingCall{{startsWith: 0, limit: 2}}, repo.getPendingCalls)
	require.Empty(t, repo.markSentCalls)
	require.Empty(t, repo.markFailedCalls)
	require.Empty(t, bot.attempts)
}

func TestOutboxRelayUseCase_WorkerFn_ReturnsMarkSentError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("can't persist sent state")
	events := []model.OutboxLinkUpdateEvent{
		makeOutboxEvent(1, "issue-1"),
	}
	repo := &fakeRelayOutboxRepo{
		responses: map[int64]pendingResponse{
			0: {events: events},
		},
		markSentErr: wantErr,
	}
	bot := &fakeRelayBotPort{}
	uc := OutboxRelayUseCase{
		logger:     testOutboxRelayLogger(),
		sender:     bot,
		outboxRepo: repo,
		limit:      2,
	}

	err := uc.workerFn(context.Background())

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, [][]int64{{1}}, repo.markSentCalls)
	require.Empty(t, repo.markFailedCalls)
	require.Equal(t, outboxEventsToRelayEvents(events), bot.attempts)
}

func TestOutboxRelayUseCase_WorkerFn_IgnoresMarkFailedErrorAndContinues(t *testing.T) {
	t.Parallel()

	events := []model.OutboxLinkUpdateEvent{
		makeOutboxEvent(1, "issue-1"),
		makeOutboxEvent(2, "issue-2"),
		makeOutboxEvent(3, "issue-3"),
	}
	repo := &fakeRelayOutboxRepo{
		responses: map[int64]pendingResponse{
			0: {events: events[:2]},
			2: {events: events[2:]},
		},
		markFailedErrs: map[int64]error{
			2: errors.New("can't mark failed"),
		},
	}
	bot := &fakeRelayBotPort{
		failures: map[string]error{
			"issue-2": errors.New("bot is down"),
		},
	}
	uc := OutboxRelayUseCase{
		logger:     testOutboxRelayLogger(),
		sender:     bot,
		outboxRepo: repo,
		limit:      2,
	}

	err := uc.workerFn(context.Background())

	require.NoError(t, err)
	require.Equal(t, [][]int64{{1}, {3}}, repo.markSentCalls)
	require.Equal(t, []markFailedCall{{
		id:     2,
		reason: "bot is down",
	}}, repo.markFailedCalls)
	require.Equal(t, outboxEventsToRelayEvents(events), bot.attempts)
}

func makeOutboxEvent(id int64, name string) model.OutboxLinkUpdateEvent {
	event := model.LinkUpdatedEvent{
		LinkID:       100 + id,
		RawURL:       "https://github.com/acme/project-outbox",
		UpdatedAt:    time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(id) * time.Minute),
		UpdateName:   name,
		UpdateAuthor: "alice",
		Description:  "description for " + name,
		ChatIDs:      []int64{501, 502},
	}
	return model.OutboxLinkUpdateEvent{
		ID:     id,
		LinkID: event.LinkID,
		Event:  event,
	}
}

func outboxEventsToRelayEvents(events []model.OutboxLinkUpdateEvent) []model.LinkUpdatedEvent {
	relayEvents := make([]model.LinkUpdatedEvent, len(events))
	for i, event := range events {
		relayEvents[i] = cloneRelayEvent(event.Event)
	}
	return relayEvents
}

func cloneOutboxEvents(events []model.OutboxLinkUpdateEvent) []model.OutboxLinkUpdateEvent {
	cloned := make([]model.OutboxLinkUpdateEvent, len(events))
	for i, event := range events {
		cloned[i] = model.OutboxLinkUpdateEvent{
			ID:     event.ID,
			LinkID: event.LinkID,
			Event:  cloneRelayEvent(event.Event),
		}
	}
	return cloned
}

func cloneRelayEvent(event model.LinkUpdatedEvent) model.LinkUpdatedEvent {
	event.ChatIDs = append([]int64(nil), event.ChatIDs...)
	return event
}
