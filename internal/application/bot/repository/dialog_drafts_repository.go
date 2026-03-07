package repository

import (
	"errors"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

var ErrDialogAlreadyStarted = errors.New("dialog was already started")
var ErrDialogWasntStarted = errors.New("can't add url, dialog was not started")
var ErrPartialFilling = errors.New("filling wasn't finished")

type DialogDraftsRepository interface {
	StartFilling(chatID int64) error
	AddURL(chatID int64, rawURL string) error
	AddTags(chatID int64, tags []string) error
	GetFilledTracking(chatID int64) (domain.Tracking, error)
	ResetFilling(chatID int64) error
}
