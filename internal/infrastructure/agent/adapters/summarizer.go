package adapters

import (
	"context"
	"log/slog"
	"strings"

	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
)

type DummySummarizer struct {
	logger   *slog.Logger
	treshold uint
}

func NewDummySummarizer(logger *slog.Logger, treshold uint) ports.Summarizer {
	return &DummySummarizer{
		logger:   logger,
		treshold: treshold,
	}
}

func (sum *DummySummarizer) Summarize(
	ctx context.Context, updateEvent model.LinkUpdatedEvent,
) (model.LinkUpdatedEvent, error) {
	if len(updateEvent.Description) <= int(sum.treshold) {
		return updateEvent, nil
	}
	var sb strings.Builder
	sum.logger.Info("summarization was invoked")
	sb.WriteString("Summarized message.")
	sb.WriteString(updateEvent.Description[:sum.treshold])
	sb.WriteString("...")
	return model.LinkUpdatedEvent{
		LinkID:       updateEvent.LinkID,
		RawURL:       updateEvent.RawURL,
		UpdatedAt:    updateEvent.UpdatedAt,
		UpdateName:   updateEvent.UpdateName,
		UpdateAuthor: updateEvent.UpdateAuthor,
		ChatIDs:      updateEvent.ChatIDs,
		Description:  sb.String(),
		Priority:     updateEvent.Priority,
	}, nil
}
