package ports

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type ReceivedEvent struct {
	Event         model.LinkUpdatedEvent
	MarkProcessed func(context.Context) error
	MarkFailed    func(context.Context, error) error
}

type ReceivedEventsReader interface {
	Start(context.Context) (<-chan ReceivedEvent, error)
}
