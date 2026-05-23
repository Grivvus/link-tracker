package ports

import (
	"context"
	"errors"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

var SummarizationError = errors.New("can't make summarization")

type Summarizer interface {
	Summarize(
		context.Context, model.LinkUpdatedEvent,
	) (model.LinkUpdatedEvent, error)
}
