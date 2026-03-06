package ports

import (
	"context"
	"errors"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
)

var ErrNoChatWithThisID = errors.New("can't find chat with this id")
var ErrChatAlreadyExist = errors.New("chat was already registered")
var ErrBadParams = errors.New("recieved params didn't match specifiaction")

type ScrapperPort interface {
	DeleteLinks(ctx context.Context, in dto.DeleteLink) (dto.DeleteLinksResponse, error)
	GetTrackedLinks(ctx context.Context, chatID int64) (dto.TrackedLinksResponse, error)
	AddLinks(ctx context.Context, in dto.AddLink) (dto.AddLinksResponse, error)
	DeleteTGChat(ctx context.Context, chatID int64) error
	RegisterTGChat(ctx context.Context, chatID int64) error
}
