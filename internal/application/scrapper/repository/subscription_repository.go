package repository

import (
	"context"
	"net/url"

	"link-tracker/internal/application/model"
	"link-tracker/internal/domain"
)

type SubscriptionRepository interface {
	Subscribe(
		ctx context.Context, link domain.Link, chatID int64, tags []string,
	) (domain.Subscription, error)
	Unsubscribe(
		ctx context.Context, link domain.Link, chatID int64,
	) (domain.Subscription, error)
	GetLink(ctx context.Context, linkID int64) (domain.Link, error)
	GetAllLinks(ctx context.Context) (<-chan domain.Link, <-chan error)
	UpdateLink(
		ctx context.Context, link domain.Link, events []model.LinkUpdatedEvent,
	) (domain.Link, error)
	FindLinkByURL(ctx context.Context, url *url.URL) (domain.Link, error)
	AllChatSubscriptions(ctx context.Context, chatID int64) ([]domain.Subscription, error)
	AllLinkSubscriptions(ctx context.Context, link domain.Link) ([]domain.Subscription, error)
}
