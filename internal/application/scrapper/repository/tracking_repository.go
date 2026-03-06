package repository

import (
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
)

type TrackingRepository interface {
	Save(t domain.Tracking) error
	RemoveByURL(chatID int64, url *url.URL)
	FindByChat(chatID int64) ([]domain.Tracking, error)
}

func NewTrackingRepository() TrackingRepository {
	return make(storage.TrackingStorage, 10)
}
