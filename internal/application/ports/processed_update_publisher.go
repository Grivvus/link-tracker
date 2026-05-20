package ports

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type ProcessedUpdatePublisher interface {
	PublishProcessedEvent(context.Context, model.LinkUpdatedEvent) error
}
