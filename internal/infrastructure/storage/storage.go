package storage

import (
	"context"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type trackingStorage map[int64]domain.Tracking

func NewTrackingStorage() repository.TrackingRepository {
	return make(trackingStorage, 10)
}

func (ts trackingStorage) Save(
	ctx context.Context, t domain.Tracking,
) (domain.Tracking, error) {
	id := ts.findPrevMaxID() + 1
	t.ID = id
	ts[id] = t
}

func (ts trackingStorage) FindByChat(
	ctx context.Context, chatID int64,
) ([]domain.Tracking, error) {
	// check if chat is registered
	ret := make([]domain.Tracking, 0)
	for _, t := range ts {
		if t.ChatID == chatID {
			ret = append(ret, t)
		}
	}
	return ret, nil
}

func (ts trackingStorage) RemoveByURL(
	ctx context.Context, chatID int64, url *url.URL,
) (domain.Tracking, error) {
}

func (ts trackingStorage) findPrevMaxID() int64 {
}

type chatStorage map[int64]struct{}

func NewChatStorage() repository.ChatRepository {
	return make(chatStorage, 10)
}

func (ch chatStorage) Register(ctx context.Context, chatID int64) error {
	if _, ok := ch[chatID]; ok {
		return repository.ErrEntityAlreadyExists
	}
	ch[chatID] = struct{}{}
	return nil
}

func (ch chatStorage) Delete(ctx context.Context, chatID int64) error {
	if _, ok := ch[chatID]; !ok {
		return repository.ErrEntityNotFound
	}
	delete(ch, chatID)
	return nil
}

func (ch chatStorage) isChatRegistered(chatID int64) bool {
	_, ok := ch[chatID]
	return ok
}
