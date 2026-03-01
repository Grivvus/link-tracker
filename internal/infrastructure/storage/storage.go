package storage

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

type TrackingStorage map[int64]domain.Tracking

func (ts TrackingStorage) Save(t domain.Tracking) error {
	return nil
}

func (ts TrackingStorage) FindByChat(chatID int64) ([]domain.Tracking, error) {
	return nil, nil
}
