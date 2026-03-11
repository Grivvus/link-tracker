package ports

import (
	"context"
	"errors"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

var ErrNoChatWithThisID = errors.New("can't find chat with this id")
var ErrNoLink = errors.New("no link was found")
var ErrChatAlreadyExist = errors.New("chat was already registered")
var ErrBadParams = errors.New("received params didn't match specification")
var ErrLinkAlreadyTracked = errors.New("this link is already tracked")

type ScrapperPort interface {
	DeleteLink(ctx context.Context, cmd model.UntrackLinkCmd) (model.TrackedLinkView, error)
	GetTrackedLinks(
		ctx context.Context, query model.ListTrackedLinksQuery,
	) ([]model.TrackedLinkView, error)
	AddLink(ctx context.Context, cmd model.TrackLinkCmd) (model.TrackedLinkView, error)
	DeleteTGChat(ctx context.Context, chatID int64) error
	RegisterTGChat(ctx context.Context, chatID int64) error
}
