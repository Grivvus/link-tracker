package orm

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type LinksMetricsRepositoryORM struct {
	db dbRunner
}

func NewLinksMetricsRepository(pool *pgxpool.Pool) repository.LinksMetricsRepository {
	return &LinksMetricsRepositoryORM{
		db: pool,
	}
}

func (r *LinksMetricsRepositoryORM) TrackedLinksCount(
	ctx context.Context,
) (gh_count, st_count uint, err error) {
	ghTrackedCount, err := r.trackedLinksCountBySource(ctx, "%github%")
	if err != nil {
		return 0, 0, wrapUnknownDBError(err)
	}

	stTrackedCount, err := r.trackedLinksCountBySource(ctx, "%stackoverflow%")
	if err != nil {
		return 0, 0, wrapUnknownDBError(err)
	}

	return uint(ghTrackedCount), uint(stTrackedCount), nil
}

func (r *LinksMetricsRepositoryORM) trackedLinksCountBySource(
	ctx context.Context, sourcePattern string,
) (int64, error) {
	return queryOne(ctx, r.db, dialect.
		Select(goqu.COUNT("*")).
		From(goqu.T("link").As("l")).
		Where(
			goqu.I("l.id").In(
				dialect.
					Select(goqu.I("s.link_id").Distinct()).
					From(goqu.T("subscription").As("s")),
			),
			goqu.I("l.url").Like(sourcePattern),
		).
		Prepared(true), pgx.RowTo[int64])
}
