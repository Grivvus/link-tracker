package ports

import (
	"context"
	"errors"

	"link-tracker/internal/application/model"
)

var SummarizationError = errors.New("can't make summarization")

type Summarizer interface {
	Summarize(
		context.Context, model.LinkUpdatedEvent,
	) (model.LinkUpdatedEvent, error)
}
