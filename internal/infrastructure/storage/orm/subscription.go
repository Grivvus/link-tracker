package orm

import (
	"context"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type SubscriptionRepositoryORM struct {
}

func NewSubscriptionRepository() repository.SubscriptionRepository {
	return &SubscriptionRepositoryORM{}
}

func (repo *SubscriptionRepositoryORM) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositoryORM) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositoryORM) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositoryORM) GetAllLinks(
	ctx context.Context,
) ([]domain.Link, error) {
}

func (repo *SubscriptionRepositoryORM) UpdateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositoryORM) FindLinkByURL(
	ctx context.Context, url *url.URL,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositoryORM) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
}

func (repo *SubscriptionRepositoryORM) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
}
