package sql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	gen "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

type LinksMetricsRepositorySQL struct {
	queries *gen.Queries
	pool    *pgxpool.Pool
}

func NewLinksMetricsRepository(pool *pgxpool.Pool) repository.LinksMetricsRepository {
	return &LinksMetricsRepositorySQL{
		pool:    pool,
		queries: gen.New(pool),
	}
}

func (r *LinksMetricsRepositorySQL) TrackedLinksCount(
	ctx context.Context,
) (gh_count, st_count uint, err error) {
	ghTrackedCount, err := r.queries.GetTrackedLinksCntGithub(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", repository.ErrUnknownDBError, err)
	}
	stTrackedCount, err := r.queries.GetTrackedLinksCntStackoverflow(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", repository.ErrUnknownDBError, err)
	}

	return uint(ghTrackedCount), uint(stTrackedCount), nil
}
