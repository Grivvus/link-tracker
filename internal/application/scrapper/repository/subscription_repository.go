package repository

import (
	"context"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type SubscriptionRepository interface {
	Subscribe(
		ctx context.Context, link domain.Link, chatID int64, tags []string,
	) (domain.Subscription, error)
	Unsubscribe(ctx context.Context, link domain.Link, chatID int64) error
	FindLinkByURL(ctx context.Context, url *url.URL) (domain.Link, error)
	AllChatSubscriptions(ctx context.Context, chatID int64) ([]domain.Subscription, error)
	AllLinkSubscriptions(ctx context.Context, link domain.Link) ([]domain.Subscription, error)
}
