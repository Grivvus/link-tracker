package sql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"link-tracker/internal/application/model"
	"link-tracker/internal/application/scrapper/repository"
	gen "link-tracker/internal/infrastructure/storage/sql/gen"
)

type NotificationOutboxRepositorySQL struct {
	queries *gen.Queries
	pool    *pgxpool.Pool
}

func NewNotificationOutboxRepository(
	pool *pgxpool.Pool,
) repository.NotificationOutboxRepository {
	return &NotificationOutboxRepositorySQL{
		queries: gen.New(pool),
		pool:    pool,
	}
}

func (repo *NotificationOutboxRepositorySQL) GetPending(
	ctx context.Context, startsWith int64, limit int32,
) ([]model.OutboxLinkUpdateEvent, error) {
	events, err := repo.queries.GetPendingWithLimit(ctx, gen.GetPendingWithLimitParams{
		ID:    startsWith,
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: can't fetch pending events - %w",
			repository.ErrUnknownDBError, err,
		)
	}
	return toModelOutboxNotifications(events)
}

func (repo *NotificationOutboxRepositorySQL) MarkSent(
	ctx context.Context, ids []int64,
) error {
	_, err := withTx(ctx, repo.pool, repo.queries, func(q *gen.Queries) (int, error) {
		for _, id := range ids {
			err := q.MarkSent(ctx, id)
			if err != nil {
				return 0, fmt.Errorf(
					"%w: can't mark event %d as sent - %w",
					repository.ErrUnknownDBError, id, err,
				)
			}
		}
		return 0, nil
	})
	return err
}

func (repo *NotificationOutboxRepositorySQL) MarkFailed(
	ctx context.Context, id int64, reason string,
) error {
	err := repo.queries.MarkFailed(ctx, gen.MarkFailedParams{
		ID: id, FailingReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		return fmt.Errorf(
			"%w: can't mark event %d as failed - %w",
			repository.ErrUnknownDBError, id, err,
		)
	}
	return nil
}
