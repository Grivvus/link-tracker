package adapters

import (
	"context"
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
)

type DummySummarizer struct {
	logger *slog.Logger
}

func NewDummySummarizer(logger *slog.Logger) ports.Summarizer {
	return &DummySummarizer{
		logger: logger,
	}
}

func (sum *DummySummarizer) Summarize(
	ctx context.Context, updateEvent model.LinkUpdatedEvent,
	treshold uint,
) (model.LinkUpdatedEvent, error) {
	var sb strings.Builder
	sum.logger.Info("summarization was invoked")
	sb.WriteString("Summarized message.")
	sb.WriteString(updateEvent.Description[:treshold])
	sb.WriteString("...")
	return model.LinkUpdatedEvent{
		LinkID:       updateEvent.LinkID,
		RawURL:       updateEvent.RawURL,
		UpdatedAt:    updateEvent.UpdatedAt,
		UpdateName:   updateEvent.UpdateName,
		UpdateAuthor: updateEvent.UpdateAuthor,
		ChatIDs:      updateEvent.ChatIDs,
		Description:  sb.String(),
	}, nil
}
