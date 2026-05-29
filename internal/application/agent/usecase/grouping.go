package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/infrastructure"
)

type UpdateMerger interface {
	Merge(int64, []model.LinkUpdatedEvent) model.LinkUpdatedEvent
}

type UpdateGrouper interface {
	Add(ctx context.Context, event ports.ReceivedEvent, chatID int64)
	Flush(context.Context) error
}

type UpdateGroup struct {
	Key      int64
	Deadline time.Time
	Events   []ports.ReceivedEvent
}

type InMemoryUpdateGrouper struct {
	mu         sync.Mutex
	logger     *slog.Logger
	window     time.Duration
	merger     UpdateMerger
	summarizer ports.Summarizer
	publisher  ports.ProcessedUpdatePublisher
	groups     map[int64]*UpdateGroup
}

func NewInMemoryUpdateGrouper(
	ctx context.Context,
	logger *slog.Logger,
	window time.Duration,
	merger UpdateMerger,
	summarizer ports.Summarizer,
	publisher ports.ProcessedUpdatePublisher,
) *InMemoryUpdateGrouper {
	grouper := &InMemoryUpdateGrouper{
		logger:     logger,
		window:     window,
		merger:     merger,
		summarizer: summarizer,
		publisher:  publisher,
		groups:     make(map[int64]*UpdateGroup),
	}
	grouper.runChecker(ctx)
	return grouper
}

func (g *InMemoryUpdateGrouper) Add(
	ctx context.Context, event ports.ReceivedEvent, forChat int64,
) {
	g.mu.Lock()
	defer g.mu.Unlock()
	group, ok := g.groups[forChat]
	if !ok {
		newGroup := &UpdateGroup{
			Key:      forChat,
			Deadline: time.Now().Add(g.window),
			Events:   make([]ports.ReceivedEvent, 0),
		}
		newGroup.Events = append(newGroup.Events, event)
		g.groups[forChat] = newGroup
		return
	}
	group.Events = append(group.Events, event)
}

func (g *InMemoryUpdateGrouper) Flush(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var errs error
	for chatID, group := range g.groups {
		err := g.flushOne(ctx, chatID, group)
		errors.Join(errs, err)
	}
	return errs
}

func (g *InMemoryUpdateGrouper) runChecker(ctx context.Context) {
	// check g.groups
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				g.mu.Lock()
				func() {
					defer g.mu.Unlock()
					for chatID, group := range g.groups {
						if group.Deadline.After(time.Now()) {
							continue
						}
						err := g.flushOne(ctx, chatID, group)
						if err != nil {
							g.logger.Error(
								"can't flush group",
								"chatID", chatID,
								"err", err,
							)
						}
					}
				}()
				time.Sleep(max(g.window/4, 100*time.Millisecond))
			}
		}
	}()
}

func (g *InMemoryUpdateGrouper) flushOne(
	ctx context.Context, chatID int64, group *UpdateGroup,
) error {
	merged := g.merger.Merge(chatID, infrastructure.Map(
		group.Events, func(e ports.ReceivedEvent) model.LinkUpdatedEvent {
			return e.Event
		},
	))
	summarized, err := g.summarizer.Summarize(ctx, merged)
	if err != nil {
		g.logger.Error("can't summarize update, fallback to unsummarized", "err", err)
		summarized = merged
	}
	err = g.publisher.PublishProcessedEvent(ctx, summarized)
	if err != nil {
		// mark every message in group as failed
		for _, event := range group.Events {
			publishErr := event.MarkFailed(ctx, err)
			if publishErr != nil {
				g.logger.Error(
					"can't mark event as failed",
					"event", event.Event,
					"err", publishErr,
				)
				return publishErr
			}
		}
	} else {
		// mark every message in group as processed
		for _, event := range group.Events {
			err := event.MarkProcessed(ctx)
			if err != nil {
				g.logger.Error("can't mark event as processed", "event", event.Event)
				return err
			}
		}
	}
	delete(g.groups, chatID)
	return nil
}

type DefaultUpdateMerger struct{}

func NewDefaultUpdateMerger() DefaultUpdateMerger {
	return DefaultUpdateMerger{}
}

func (m DefaultUpdateMerger) Merge(chatID int64, events []model.LinkUpdatedEvent) model.LinkUpdatedEvent {
	if len(events) == 0 {
		return model.LinkUpdatedEvent{}
	}
	maxPriority := events[0].Priority
	maxUpdTime := events[0].UpdatedAt
	var descriptionBuilder strings.Builder
	var urlBuilder strings.Builder
	var authorsBuilder strings.Builder
	var nameBuilder strings.Builder
	for i, event := range events {
		maxPriority = max(maxPriority, event.Priority)
		if event.UpdatedAt.After(maxUpdTime) {
			maxUpdTime = event.UpdatedAt
		}
		descriptionBuilder.WriteString(fmt.Sprintf("%d: %s\n", i+1, event.Description))
		urlBuilder.WriteString(fmt.Sprintf("%d: %s ||", i+1, event.RawURL))
		authorsBuilder.WriteString(fmt.Sprintf("%d: %s ||", i+1, event.UpdateAuthor))
		nameBuilder.WriteString(fmt.Sprintf("%d: %s ||", i+1, event.UpdateName))
	}
	urlBuilder.WriteString("\n\n")
	authorsBuilder.WriteString("\n\n")
	nameBuilder.WriteString("\n\n")

	return model.LinkUpdatedEvent{
		RawURL:       urlBuilder.String(),
		UpdatedAt:    maxUpdTime,
		Priority:     maxPriority,
		UpdateName:   nameBuilder.String(),
		UpdateAuthor: authorsBuilder.String(),
		Description:  descriptionBuilder.String(),
		ChatIDs:      []int64{chatID},
	}
}
