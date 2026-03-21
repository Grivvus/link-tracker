package sql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	gen "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

func toDomainLink(link gen.Link) (domain.Link, error) {
	domLink, err := domain.NewLink(link.Url)
	if err != nil {
		// Data comes from DB and must already be validated.
		return domain.Link{}, fmt.Errorf("unreachable, error always should be nil")
	}
	domLink.ID = link.ID
	domLink.LastUpdate = link.LastUpdate.Time

	return domLink, nil
}

func withTx[T any](
	ctx context.Context, pool *pgxpool.Pool, queries *gen.Queries, fn func(queries *gen.Queries) (T, error),
) (T, error) {
	var zero T

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	qtx := queries.WithTx(tx)
	if err != nil {
		return zero, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	value, err := fn(qtx)
	if err != nil {
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}

	return value, nil
}
