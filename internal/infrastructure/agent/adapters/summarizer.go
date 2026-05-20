package adapters

import (
	"context"
	"log/slog"

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
	sum.logger.Info("summarization was invoked")
	summarized := "Summarized message."
	if len(updateEvent.Description) > int(treshold) {
		summarized += updateEvent.Description[:treshold]
	} else {
		summarized += updateEvent.Description
	}
	return model.LinkUpdatedEvent{
		LinkID:       updateEvent.LinkID,
		RawURL:       updateEvent.RawURL,
		UpdatedAt:    updateEvent.UpdatedAt,
		UpdateName:   updateEvent.UpdateName,
		UpdateAuthor: updateEvent.UpdateAuthor,
		ChatIDs:      updateEvent.ChatIDs,
		Description:  summarized,
	}, nil
}
