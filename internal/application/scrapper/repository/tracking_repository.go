package repository

import (
	"context"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type TrackingRepository interface {
	Save(ctx context.Context, t domain.Tracking) (domain.Tracking, error)
	RemoveByURL(ctx context.Context, chatID int64, url *url.URL) (domain.Tracking, error)
	FindByChat(ctx context.Context, chatID int64) ([]domain.Tracking, error)
	GetAllTrackings(ctx context.Context) ([]domain.Tracking, error)
}
