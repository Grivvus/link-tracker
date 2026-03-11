package ports

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type BotPort interface {
	NotifyAboutUpdatedLinks(ctx context.Context, event model.LinkUpdatedEvent) error
}
