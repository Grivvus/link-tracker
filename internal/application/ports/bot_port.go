package ports

import (
	"context"

	"link-tracker/internal/application/model"
)

type BotPort interface {
	NotifyAboutUpdatedLinks(ctx context.Context, event model.LinkUpdatedEvent) error
}
