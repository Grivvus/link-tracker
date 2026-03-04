package ports

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
)

type BotPort interface {
	NotifyAboutUpdatedLinks(ctx context.Context, in dto.LinkUpdate) error
}
