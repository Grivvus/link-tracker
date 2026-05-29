package orm

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"link-tracker/internal/application/scrapper/repository"
	"link-tracker/internal/domain"
)

var dialect = goqu.Dialect("postgres")

type sqlizer interface {
	ToSQL() (string, []interface{}, error)
}

type dbRunner interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

func buildSQL(ds sqlizer) (string, []any, error) {
	stmt, args, err := ds.ToSQL()
	if err != nil {
		return "", nil, fmt.Errorf("%w: detail - %v", repository.ErrUnknownDBError, err)
	}
	return stmt, args, nil
}

func execDataset(ctx context.Context, db dbRunner, ds sqlizer) error {
	stmt, args, err := buildSQL(ds)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, stmt, args...)
	return err
}

func queryOne[T any](
	ctx context.Context, db dbRunner, ds sqlizer, collector pgx.RowToFunc[T],
) (T, error) {
	var zero T

	stmt, args, err := buildSQL(ds)
	if err != nil {
		return zero, err
	}

	rows, err := db.Query(ctx, stmt, args...)
	if err != nil {
		return zero, err
	}
	defer rows.Close()

	return pgx.CollectOneRow(rows, collector)
}

func queryMany[T any](
	ctx context.Context, db dbRunner, ds sqlizer, collector pgx.RowToFunc[T],
) ([]T, error) {
	stmt, args, err := buildSQL(ds)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, collector)
}

func withTx[T any](
	ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) (T, error),
) (T, error) {
	var zero T

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, wrapUnknownDBError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	value, err := fn(tx)
	if err != nil {
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, wrapUnknownDBError(err)
	}

	return value, nil
}

func wrapUnknownDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf(
			"%w: msg - %s, pgcode: %s",
			repository.ErrUnknownDBError, pgErr.Message, pgErr.Code,
		)
	}

	return fmt.Errorf("%w: msg - %v", repository.ErrUnknownDBError, err)
}

func wrapDuplicateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return repository.ErrEntityAlreadyExists
	}

	return wrapUnknownDBError(err)
}

func requireURL(rawURL *url.URL) (string, error) {
	if rawURL == nil {
		return "", fmt.Errorf("%w: nil link url", repository.ErrEntityNotFound)
	}

	return rawURL.String(), nil
}

func toDomainLink(dbLink Link) (domain.Link, error) {
	domLink, err := domain.NewLink(dbLink.URL)
	if err != nil {
		// Data comes from DB and must already be validated.
		return domain.Link{}, fmt.Errorf("unreachable, error always should be nil")
	}
	domLink.ID = dbLink.ID
	domLink.LastUpdate = dbLink.LastUpdate.Time

	return domLink, nil
}

func toDomainLinks(dbLinks []Link) ([]domain.Link, error) {
	links := make([]domain.Link, len(dbLinks))
	for i, dbLink := range dbLinks {
		link, err := toDomainLink(dbLink)
		if err != nil {
			return nil, err
		}
		links[i] = link
	}

	return links, nil
}

func toDomainSubscription(dbSub Subscription, tags []string) domain.Subscription {
	return domain.NewSubscription(dbSub.ID, dbSub.LinkID, dbSub.ChatID, tags)
}
