package sql

import (
	"context"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	gen "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

type SubscriptionRepositorySQL struct {
	queries *gen.Queries
	pool    *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) repository.SubscriptionRepository {
	return &SubscriptionRepositorySQL{
		queries: gen.New(pool),
		pool:    pool,
	}
}

func (repo *SubscriptionRepositorySQL) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositorySQL) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositorySQL) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositorySQL) GetAllLinks(
	ctx context.Context,
) ([]domain.Link, error) {
}

func (repo *SubscriptionRepositorySQL) UpdateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositorySQL) FindLinkByURL(
	ctx context.Context, url *url.URL,
) (domain.Link, error) {
}

func (repo *SubscriptionRepositorySQL) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
}

func (repo *SubscriptionRepositorySQL) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
}
