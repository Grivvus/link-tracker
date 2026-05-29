package usecase

import (
	"context"
	"slices"
	"strings"
	"sync"

	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/infrastructure/agent"
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
	filter      UpdateFilter
	prioritizer Prioritizer
	grouper     UpdateGrouper
}

func NewEventUpdateProcessorUseCase(
	filter UpdateFilter,
	prioritizer Prioritizer,
	grouper UpdateGrouper,
) EventUpdateProcessorUseCase {
	return EventUpdateProcessorUseCase{
		filter:      filter,
		prioritizer: prioritizer,
		grouper:     grouper,
	}
}

func (uc EventUpdateProcessorUseCase) Process(
	ctx context.Context, updateEvent ports.ReceivedEvent,
) error {
	if !uc.filter.Allow(updateEvent.Event) {
		return updateEvent.MarkProcessed(ctx)
	}

	updateEvent.Event.Priority = uc.prioritizer.Prioritize(updateEvent.Event)
	if len(updateEvent.Event.ChatIDs) == 0 {
		return updateEvent.MarkProcessed(ctx)
	}
	receipt := newFanoutTracker(
		len(updateEvent.Event.ChatIDs),
		updateEvent.MarkProcessed,
		updateEvent.MarkFailed,
	)
	for _, chatID := range updateEvent.Event.ChatIDs {
		eventForChat := updateEvent
		eventForChat.Event.ChatIDs = []int64{chatID}
		eventForChat.MarkProcessed = receipt.MarkProcessed
		eventForChat.MarkFailed = receipt.MarkFailed
		uc.grouper.Add(ctx, eventForChat, chatID)
	}
	return nil
}

type fanoutTracker struct {
	mu            sync.Mutex
	remaining     int
	done          bool
	markProcessed func(context.Context) error
	markFailed    func(context.Context, error) error
}

func newFanoutTracker(
	remaining int,
	markProcessed func(context.Context) error,
	markFailed func(context.Context, error) error,
) *fanoutTracker {
	return &fanoutTracker{
		remaining:     remaining,
		markProcessed: markProcessed,
		markFailed:    markFailed,
	}
}

func (r *fanoutTracker) MarkProcessed(ctx context.Context) error {
	r.mu.Lock()
	if r.done {
		r.mu.Unlock()
		return nil
	}
	r.remaining--
	if r.remaining > 0 {
		r.mu.Unlock()
		return nil
	}
	r.done = true
	markProcessed := r.markProcessed
	r.mu.Unlock()
	return markProcessed(ctx)
}

func (r *fanoutTracker) MarkFailed(ctx context.Context, err error) error {
	r.mu.Lock()
	if r.done {
		r.mu.Unlock()
		return nil
	}
	r.done = true
	markFailed := r.markFailed
	r.mu.Unlock()
	return markFailed(ctx, err)
}
