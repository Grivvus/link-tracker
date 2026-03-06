package repository

import (
	"context"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
)

type TrackingRepository interface {
	Save(ctx context.Context, t domain.Tracking) (domain.Tracking, error)
	RemoveByURL(ctx context.Context, chatID int64, url *url.URL) (domain.Tracking, error)
	FindByChat(ctx context.Context, chatID int64) ([]domain.Tracking, error)
}

func NewTrackingRepository() TrackingRepository {
	return make(storage.TrackingStorage, 10)
}
