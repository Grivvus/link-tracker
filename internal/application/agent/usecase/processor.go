package usecase

import (
	"context"
	"slices"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent"
)

type Prioritizer interface {
	Prioritize(model.LinkUpdatedEvent) model.Priority
}

type prioritizer struct {
	settings agent.AgentSettings
}

func NewPrioritizer(s agent.AgentSettings) Prioritizer {
	return prioritizer{settings: s}
}

func (p prioritizer) Prioritize(update model.LinkUpdatedEvent) model.Priority {
	for _, hkw := range p.settings.Prioritization.HighKeywords {
		if strings.Contains(update.Description, hkw) {
			return model.PriorityHigh
		}
	}

	for _, lkw := range p.settings.Prioritization.LowKeywords {
		if strings.Contains(update.Description, lkw) {
			return model.PriorityLow
		}
	}

	return model.PriorityMedium
}

type UpdateFilter interface {
	Allow(model.LinkUpdatedEvent) bool
}

type filter struct {
	settings agent.AgentSettings
}

func (f filter) Allow(updateEvent model.LinkUpdatedEvent) bool {
	if len(updateEvent.Description) < f.settings.Filtering.MinLength {
		return false
	}

	if slices.Contains(f.settings.Filtering.ExcludedAuthors, updateEvent.UpdateAuthor) {
		return false
	}

	for _, stopWord := range f.settings.Filtering.StopWords {
		if strings.Contains(updateEvent.Description, stopWord) {
			return false
		}
	}
	return true
}

func NewUpdateFilter(settings agent.AgentSettings) UpdateFilter {
	return filter{settings: settings}
}

type EventUpdateProcessorUseCase struct {
	filter                 UpdateFilter
	prioritizer            Prioritizer
	summarizer             ports.Summarizer
	publisher              ports.ProcessedUpdatePublisher
	summarizationThreshold uint
}

func NewEventUpdateProcessorUseCase(
	filter UpdateFilter, summarizer ports.Summarizer,
	prioritizer Prioritizer,
	publisher ports.ProcessedUpdatePublisher,
	summarizationThreshold uint,
) EventUpdateProcessorUseCase {
	return EventUpdateProcessorUseCase{
		filter:                 filter,
		summarizer:             summarizer,
		prioritizer:            prioritizer,
		publisher:              publisher,
		summarizationThreshold: summarizationThreshold,
	}
}

func (uc EventUpdateProcessorUseCase) Process(
	ctx context.Context, updateEvent model.LinkUpdatedEvent,
) error {
	if !uc.filter.Allow(updateEvent) {
		return nil
	}

	updateEvent.Priority = uc.prioritizer.Prioritize(updateEvent)

	if uc.shouldSummarize(updateEvent) {
		var err error
		updateEvent, err = uc.summarizer.Summarize(ctx, updateEvent, uc.summarizationThreshold)
		if err != nil {
			return err
		}
	}

	return uc.publisher.PublishProcessedEvent(ctx, updateEvent)
}

func (uc EventUpdateProcessorUseCase) shouldSummarize(updateEvent model.LinkUpdatedEvent) bool {
	return len(updateEvent.Description) >= int(uc.summarizationThreshold)
}
