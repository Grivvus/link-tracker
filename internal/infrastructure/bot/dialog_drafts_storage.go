package bot

import (
	"log/slog"
	"net/url"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type dialogDraftsStorage struct {
	logger  *slog.Logger
	mu      sync.Mutex
	storage map[int64]domain.Tracking
}

func newDialogDraftsStorage(logger *slog.Logger) repository.DialogDraftsRepository {
	return &dialogDraftsStorage{
		logger:  logger,
		mu:      sync.Mutex{},
		storage: make(map[int64]domain.Tracking, 10),
	}
}

func (st *dialogDraftsStorage) StartFilling(chatID int64) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.storage[chatID]; ok {
		return repository.ErrDialogAlreadyStarted
	}
	st.storage[chatID] = domain.Tracking{}
	return nil
}

func (st *dialogDraftsStorage) AddURL(chatID int64, rawURL string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.storage[chatID]; !ok {
		return repository.ErrDialogWasntStarted
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return domain.NewErrBadURL(rawURL)
	}
	st.storage[chatID] = domain.Tracking{
		URL: parsedURL,
	}
	return nil
}

func (st *dialogDraftsStorage) AddTags(chatID int64, tags []string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.storage[chatID]; !ok {
		return repository.ErrDialogWasntStarted
	}
	st.storage[chatID] = domain.Tracking{
		Tags:   tags,
		URL:    st.storage[chatID].URL,
		ChatID: chatID,
	}
	return nil
}

func (st *dialogDraftsStorage) GetFilledTracking(chatID int64) (domain.Tracking, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	tracking, ok := st.storage[chatID]
	if !ok {
		return domain.Tracking{}, repository.ErrDialogWasntStarted
	}
	if tracking.URL == nil || tracking.Tags == nil {
		return domain.Tracking{}, repository.ErrPartialFilling
	}
	return tracking, nil
}

func (st *dialogDraftsStorage) ResetFilling(chatID int64) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.storage[chatID]; !ok {
		return repository.ErrDialogWasntStarted
	}
	delete(st.storage, chatID)
	return nil
}
