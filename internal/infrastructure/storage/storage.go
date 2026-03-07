package storage

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type Storage struct {
	trackingStorage      map[int64]domain.Tracking
	chatStorage          map[int64]struct{}
	currentMaxTrackingID atomic.Int64
	rwmuChat             sync.RWMutex
	rwmuTracking         sync.RWMutex
}

var storage *Storage = nil

func NewTrackingStorage() repository.TrackingRepository {
	if storage == nil {
		storage = &Storage{
			trackingStorage: make(map[int64]domain.Tracking, 10),
			chatStorage:     make(map[int64]struct{}, 10),
		}
	}
	return storage
}

func NewChatStorage() repository.ChatRepository {
	if storage == nil {
		storage = &Storage{
			trackingStorage: make(map[int64]domain.Tracking, 10),
			chatStorage:     make(map[int64]struct{}, 10),
		}
	}
	return storage
}

func (st *Storage) Save(
	ctx context.Context, t domain.Tracking,
) (domain.Tracking, error) {
	id := st.getNextTrackingID()
	t.ID = id
	st.rwmuTracking.Lock()
	defer st.rwmuTracking.Unlock()
	st.trackingStorage[id] = t
	return t, nil
}

func (st *Storage) FindByChat(
	ctx context.Context, chatID int64,
) ([]domain.Tracking, error) {
	// check if chat is registered
	ret := make([]domain.Tracking, 0)
	for _, tracking := range st.trackingStorage {
		if tracking.ChatID == chatID {
			ret = append(ret, tracking)
		}
	}
	return ret, nil
}

func (st *Storage) RemoveByURL(
	ctx context.Context, chatID int64, url *url.URL,
) (domain.Tracking, error) {
	st.rwmuChat.RLock()
	if _, ok := st.chatStorage[chatID]; !ok {
		return domain.Tracking{}, fmt.Errorf(
			"%w: no chat with this id", repository.ErrEntityNotFound,
		)
	}
	st.rwmuChat.RUnlock()

	st.rwmuTracking.Lock()
	defer st.rwmuTracking.Unlock()
	for _, tracking := range st.trackingStorage {
		if tracking.ChatID == chatID && tracking.URL.String() == url.String() {
			delete(st.trackingStorage, tracking.ID)
			return tracking, nil
		}
	}
	return domain.Tracking{}, fmt.Errorf(
		"%w: no url was found", repository.ErrEntityNotFound,
	)
}

func (st *Storage) getNextTrackingID() int64 {
	newID := st.currentMaxTrackingID.Add(1)
	return newID
}

func (st *Storage) Register(ctx context.Context, chatID int64) error {
	if _, ok := st.chatStorage[chatID]; ok {
		return repository.ErrEntityAlreadyExists
	}
	st.chatStorage[chatID] = struct{}{}
	return nil
}

func (st *Storage) Delete(ctx context.Context, chatID int64) error {
	st.rwmuChat.Lock()
	defer st.rwmuChat.Unlock()
	if _, ok := st.chatStorage[chatID]; !ok {
		return repository.ErrEntityNotFound
	}
	delete(st.chatStorage, chatID)
	return nil
}

func (st *Storage) isChatRegistered(chatID int64) bool {
	st.rwmuChat.RLock()
	defer st.rwmuChat.RUnlock()
	_, ok := st.chatStorage[chatID]
	return ok
}
