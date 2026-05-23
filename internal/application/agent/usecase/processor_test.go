package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent"
)

type fakeGrouperCall struct {
	event  ports.ReceivedEvent
	chatID int64
}

type fakeGrouper struct {
	calls []fakeGrouperCall
}

func (f *fakeGrouper) Add(_ context.Context, event ports.ReceivedEvent, chatID int64) {
	f.calls = append(f.calls, fakeGrouperCall{
		event:  event,
		chatID: chatID,
	})
}

func (f *fakeGrouper) Flush(context.Context) error {
	return nil
}

type testReceipt struct {
	processed int
	failed    int
	failCause error
}

func (r *testReceipt) receivedEvent(event model.LinkUpdatedEvent) ports.ReceivedEvent {
	return ports.ReceivedEvent{
		Event: event,
		MarkProcessed: func(context.Context) error {
			r.processed++
			return nil
		},
		MarkFailed: func(_ context.Context, err error) error {
			r.failed++
			r.failCause = err
			return nil
		},
	}
}

func TestEventUpdateProcessorUseCase_Process_FiltersByStopWords(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		description string
		wantGrouped bool
	}{
		{
			name:        "filters update containing stop word",
			description: "text with blocked word",
			wantGrouped: false,
		},
		{
			name:        "passes update without stop words",
			description: "text without forbidden words",
			wantGrouped: true,
		},
	}

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Filtering.StopWords = []string{"blocked"}
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
				event.Description = tc.description
			})
			grouper := &fakeGrouper{}
			processor := newTestProcessor(settings, grouper)
			receipt := &testReceipt{}

			err := processor.Process(context.Background(), receipt.receivedEvent(event))

			require.NoError(t, err)
			if tc.wantGrouped {
				require.Zero(t, receipt.processed)
				requireGroupedByChats(t, event, grouper.calls)
				return
			}
			require.Equal(t, 1, receipt.processed)
			require.Empty(t, grouper.calls)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_FiltersByExcludedAuthors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		author      string
		wantGrouped bool
	}{
		{
			name:        "filters update from excluded author",
			author:      "ignored-author",
			wantGrouped: false,
		},
		{
			name:        "passes update from allowed author",
			author:      "allowed-author",
			wantGrouped: true,
		},
	}

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Filtering.ExcludedAuthors = []string{"ignored-author"}
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
				event.UpdateAuthor = tc.author
			})
			grouper := &fakeGrouper{}
			processor := newTestProcessor(settings, grouper)
			receipt := &testReceipt{}

			err := processor.Process(context.Background(), receipt.receivedEvent(event))

			require.NoError(t, err)
			if tc.wantGrouped {
				require.Zero(t, receipt.processed)
				requireGroupedByChats(t, event, grouper.calls)
				return
			}
			require.Equal(t, 1, receipt.processed)
			require.Empty(t, grouper.calls)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_FiltersByMinimumDescriptionLength(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		description string
		wantGrouped bool
	}{
		{
			name:        "filters update shorter than minimum length",
			description: "too short",
			wantGrouped: false,
		},
		{
			name:        "passes update matching minimum length",
			description: "description long enough",
			wantGrouped: true,
		},
	}

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Filtering.MinLength = 20
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
				event.Description = tc.description
			})
			grouper := &fakeGrouper{}
			processor := newTestProcessor(settings, grouper)
			receipt := &testReceipt{}

			err := processor.Process(context.Background(), receipt.receivedEvent(event))

			require.NoError(t, err)
			if tc.wantGrouped {
				require.Zero(t, receipt.processed)
				requireGroupedByChats(t, event, grouper.calls)
				return
			}
			require.Equal(t, 1, receipt.processed)
			require.Empty(t, grouper.calls)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_GroupsValidUpdateByChats(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings()
	event := testLinkUpdatedEvent()
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))

	require.NoError(t, err)
	require.Zero(t, receipt.processed)
	requireGroupedByChats(t, event, grouper.calls)
}

func TestEventUpdateProcessorUseCase_Process_AssignsHighPriority(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Prioritization.HighKeywords = []string{"critical", "breaking"}
		settings.Prioritization.LowKeywords = []string{"minor"}
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "critical production issue"
	})
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))

	require.NoError(t, err)
	requireGroupedWithPriority(t, model.PriorityHigh, grouper.calls)
}

func TestEventUpdateProcessorUseCase_Process_AssignsLowPriority(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Prioritization.HighKeywords = []string{"critical", "breaking"}
		settings.Prioritization.LowKeywords = []string{"minor", "docs"}
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "minor docs update"
	})
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))

	require.NoError(t, err)
	requireGroupedWithPriority(t, model.PriorityLow, grouper.calls)
}

func TestEventUpdateProcessorUseCase_Process_AssignsMediumPriorityByDefault(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Prioritization.HighKeywords = []string{"critical"}
		settings.Prioritization.LowKeywords = []string{"minor"}
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "regular update"
	})
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))

	require.NoError(t, err)
	requireGroupedWithPriority(t, model.PriorityMedium, grouper.calls)
}

func TestEventUpdateProcessorUseCase_Process_AssignsHighPriorityWhenHighAndLowKeywordsMatch(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Prioritization.HighKeywords = []string{"critical"}
		settings.Prioritization.LowKeywords = []string{"minor"}
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "critical minor update"
	})
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))

	require.NoError(t, err)
	requireGroupedWithPriority(t, model.PriorityHigh, grouper.calls)
}

func TestEventUpdateProcessorUseCase_Process_MarksOriginalProcessedAfterAllChatGroupsProcessed(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings()
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.ChatIDs = []int64{101, 202, 303}
	})
	grouper := &fakeGrouper{}
	processor := newTestProcessor(settings, grouper)
	receipt := &testReceipt{}

	err := processor.Process(context.Background(), receipt.receivedEvent(event))
	require.NoError(t, err)
	require.Len(t, grouper.calls, 3)

	require.NoError(t, grouper.calls[0].event.MarkProcessed(context.Background()))
	require.Zero(t, receipt.processed)
	require.NoError(t, grouper.calls[1].event.MarkProcessed(context.Background()))
	require.Zero(t, receipt.processed)
	require.NoError(t, grouper.calls[2].event.MarkProcessed(context.Background()))
	require.Equal(t, 1, receipt.processed)

	require.NoError(t, grouper.calls[0].event.MarkProcessed(context.Background()))
	require.Equal(t, 1, receipt.processed)
}

func newTestProcessor(
	settings agent.AgentSettings,
	grouper usecase.UpdateGrouper,
) usecase.EventUpdateProcessorUseCase {
	return usecase.NewEventUpdateProcessorUseCase(
		usecase.NewUpdateFilter(settings),
		usecase.NewPrioritizer(settings),
		grouper,
	)
}

func requireGroupedByChats(
	t *testing.T,
	expected model.LinkUpdatedEvent,
	actual []fakeGrouperCall,
) {
	t.Helper()

	require.Len(t, actual, len(expected.ChatIDs))
	for i, chatID := range expected.ChatIDs {
		require.Equal(t, chatID, actual[i].chatID)
		expectedForChat := expected
		expectedForChat.ChatIDs = []int64{chatID}
		require.Equal(t, expectedForChat, actual[i].event.Event)
	}
}

func requireGroupedWithPriority(
	t *testing.T,
	expected model.Priority,
	actual []fakeGrouperCall,
) {
	t.Helper()

	require.NotEmpty(t, actual)
	for _, call := range actual {
		require.Equal(t, expected, call.event.Event.Priority)
	}
}

func testAgentSettings(opts ...func(*agent.AgentSettings)) agent.AgentSettings {
	settings := agent.AgentSettings{
		Filtering: agent.Filtering{
			MinLength: 1,
		},
	}
	for _, opt := range opts {
		opt(&settings)
	}
	return settings
}

func testLinkUpdatedEvent(opts ...func(*model.LinkUpdatedEvent)) model.LinkUpdatedEvent {
	event := model.LinkUpdatedEvent{
		LinkID:       1,
		RawURL:       "https://github.com/example/repo",
		UpdatedAt:    time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC),
		UpdateName:   "Issue title",
		UpdateAuthor: "allowed-author",
		Description:  "valid update description",
		ChatIDs:      []int64{101, 202},
	}
	for _, opt := range opts {
		opt(&event)
	}
	return event
}
