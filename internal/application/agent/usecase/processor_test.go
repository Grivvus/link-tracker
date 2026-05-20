package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent"
)

type fakeProcessedUpdatePublisher struct {
	events []model.LinkUpdatedEvent
	err    error
}

func (f *fakeProcessedUpdatePublisher) PublishProcessedEvent(
	_ context.Context, event model.LinkUpdatedEvent,
) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}

type fakeSummarizer struct {
	calls     int
	summarize func(model.LinkUpdatedEvent) model.LinkUpdatedEvent
	err       error
}

func (f *fakeSummarizer) Summarize(
	_ context.Context, event model.LinkUpdatedEvent, treshold uint,
) (model.LinkUpdatedEvent, error) {
	f.calls++
	if f.err != nil {
		return model.LinkUpdatedEvent{}, f.err
	}
	if f.summarize != nil {
		return f.summarize(event), nil
	}
	event.Description = "summarized"
	return event, nil
}

func TestEventUpdateProcessorUseCase_Process_FiltersByStopWords(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		description   string
		wantPublished bool
	}{
		{
			name:          "filters update containing stop word",
			description:   "text with blocked word",
			wantPublished: false,
		},
		{
			name:          "passes update without stop words",
			description:   "text without forbidden words",
			wantPublished: true,
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
			summarizer := &fakeSummarizer{}
			publisher := &fakeProcessedUpdatePublisher{}
			processor := newTestProcessor(settings, summarizer, publisher)

			err := processor.Process(context.Background(), event)

			require.NoError(t, err)
			require.Zero(t, summarizer.calls)
			if tc.wantPublished {
				require.Equal(t, []model.LinkUpdatedEvent{event}, publisher.events)
				return
			}
			require.Empty(t, publisher.events)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_FiltersByExcludedAuthors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		author        string
		wantPublished bool
	}{
		{
			name:          "filters update from excluded author",
			author:        "ignored-author",
			wantPublished: false,
		},
		{
			name:          "passes update from allowed author",
			author:        "allowed-author",
			wantPublished: true,
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
			summarizer := &fakeSummarizer{}
			publisher := &fakeProcessedUpdatePublisher{}
			processor := newTestProcessor(settings, summarizer, publisher)

			err := processor.Process(context.Background(), event)

			require.NoError(t, err)
			require.Zero(t, summarizer.calls)
			if tc.wantPublished {
				require.Equal(t, []model.LinkUpdatedEvent{event}, publisher.events)
				return
			}
			require.Empty(t, publisher.events)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_FiltersByMinimumDescriptionLength(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		description   string
		wantPublished bool
	}{
		{
			name:          "filters update shorter than minimum length",
			description:   "too short",
			wantPublished: false,
		},
		{
			name:          "passes update matching minimum length",
			description:   "description long enough",
			wantPublished: true,
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
			summarizer := &fakeSummarizer{}
			publisher := &fakeProcessedUpdatePublisher{}
			processor := newTestProcessor(settings, summarizer, publisher)

			err := processor.Process(context.Background(), event)

			require.NoError(t, err)
			require.Zero(t, summarizer.calls)
			if tc.wantPublished {
				require.Equal(t, []model.LinkUpdatedEvent{event}, publisher.events)
				return
			}
			require.Empty(t, publisher.events)
		})
	}
}

func TestEventUpdateProcessorUseCase_Process_PublishesValidUpdate(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Summarization.Threshold = 1000
	})
	event := testLinkUpdatedEvent()
	summarizer := &fakeSummarizer{}
	publisher := &fakeProcessedUpdatePublisher{}
	processor := newTestProcessor(settings, summarizer, publisher)

	err := processor.Process(context.Background(), event)

	require.NoError(t, err)
	require.Zero(t, summarizer.calls)
	require.Equal(t, []model.LinkUpdatedEvent{event}, publisher.events)
}

func TestEventUpdateProcessorUseCase_Process_SummarizesLongText(t *testing.T) {
	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Summarization.Threshold = 10
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "this text is longer than threshold"
	})
	summarizer := &fakeSummarizer{
		summarize: func(event model.LinkUpdatedEvent) model.LinkUpdatedEvent {
			event.Description = "short summary"
			return event
		},
	}
	publisher := &fakeProcessedUpdatePublisher{}
	processor := newTestProcessor(settings, summarizer, publisher)

	err := processor.Process(context.Background(), event)

	require.NoError(t, err)
	require.Equal(t, 1, summarizer.calls)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "short summary", publisher.events[0].Description)
	require.NotEqual(t, event.Description, publisher.events[0].Description)
}

func TestEventUpdateProcessorUseCase_Process_PassesShortTextWithoutSummarization(t *testing.T) {
	t.Parallel()

	settings := testAgentSettings(func(settings *agent.AgentSettings) {
		settings.Summarization.Threshold = 1000
	})
	event := testLinkUpdatedEvent(func(event *model.LinkUpdatedEvent) {
		event.Description = "short text"
	})
	summarizer := &fakeSummarizer{}
	publisher := &fakeProcessedUpdatePublisher{}
	processor := newTestProcessor(settings, summarizer, publisher)

	err := processor.Process(context.Background(), event)

	require.NoError(t, err)
	require.Zero(t, summarizer.calls)
	require.Equal(t, []model.LinkUpdatedEvent{event}, publisher.events)
}

func newTestProcessor(
	settings agent.AgentSettings,
	summarizer *fakeSummarizer,
	publisher *fakeProcessedUpdatePublisher,
) usecase.EventUpdateProcessorUseCase {
	return usecase.NewEventUpdateProcessorUseCase(
		usecase.NewUpdateFilter(settings),
		summarizer,
		publisher,
		settings.Summarization.Threshold,
	)
}

func testAgentSettings(opts ...func(*agent.AgentSettings)) agent.AgentSettings {
	settings := agent.AgentSettings{
		Filtering: agent.Filtering{
			MinLength: 1,
		},
		Summarization: agent.Summarization{
			Threshold: 1000,
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
