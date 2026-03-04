package ports

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
)

type ScrapperPort interface {
	DeleteLinks(ctx context.Context, chatID int64, in dto.DeleteLink) (dto.DeleteLinksResponse, error)
	GetTrackedLinks(ctx context.Context, chatID int64) (dto.TrackedLinksResponse, error)
	AddLinks(ctx context.Context, chatID int64, in dto.AddLink) (dto.AddLinksResponse, error)
	DeleteTGChat(ctx context.Context, in dto.DeleteChat) error
	RegisterTGChat(ctx context.Context, in dto.RegisterChat) error
}
