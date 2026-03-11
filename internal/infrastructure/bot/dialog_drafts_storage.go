package bot

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type dialogDraftsStorage struct {
	logger  *slog.Logger
	mu      sync.Mutex
	storage map[int64]model.TrackLinkCmd
}

func newDialogDraftsStorage(logger *slog.Logger) repository.DialogDraftsRepository {
	return &dialogDraftsStorage{
		logger:  logger,
		mu:      sync.Mutex{},
		storage: make(map[int64]model.TrackLinkCmd, 10),
	}
}

func (st *dialogDraftsStorage) StartFilling(chatID int64) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.storage[chatID] = model.TrackLinkCmd{}
	return nil
}

func (st *dialogDraftsStorage) AddURL(chatID int64, rawURL string) error {
	if !validURL(rawURL) {
		return fmt.Errorf("invalid url: %s", rawURL)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.storage[chatID]; !ok {
		return repository.ErrDialogWasntStarted
	}
	st.storage[chatID] = model.TrackLinkCmd{
		RawURL: rawURL,
	}
	return nil
}

func (st *dialogDraftsStorage) AddTags(chatID int64, tags []string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if draft, ok := st.storage[chatID]; !ok || draft.RawURL == "" {
		return repository.ErrDialogWasntStarted
	}
	st.storage[chatID] = model.TrackLinkCmd{
		Tags:   tags,
		RawURL: st.storage[chatID].RawURL,
		ChatID: chatID,
	}
	return nil
}

func (st *dialogDraftsStorage) GetFilledTracking(chatID int64) (model.TrackLinkCmd, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	trackLink, ok := st.storage[chatID]
	if !ok {
		return model.TrackLinkCmd{}, repository.ErrDialogWasntStarted
	}
	if trackLink.RawURL == "" || trackLink.Tags == nil {
		return model.TrackLinkCmd{}, repository.ErrPartialFilling
	}
	return trackLink, nil
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

func validURL(rawURL string) bool {
	return strings.Contains(rawURL, ".") &&
		strings.Contains(rawURL, "/") &&
		!strings.Contains(rawURL, " ")
}
