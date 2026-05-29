package ports

import (
	"context"

	"link-tracker/internal/application/model"
)

type ProcessedUpdatePublisher interface {
	PublishProcessedEvent(context.Context, model.LinkUpdatedEvent) error
}
