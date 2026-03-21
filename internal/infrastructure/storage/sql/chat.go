package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	gen "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

type ChatRepositorySQL struct {
	queries *gen.Queries
	pool    *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) repository.ChatRepository {
	return &ChatRepositorySQL{
		queries: gen.New(pool),
		pool:    pool,
	}
}

func (repo *ChatRepositorySQL) Register(ctx context.Context, chatID int64) error {
	err := repo.queries.RegisterChat(ctx, chatID)
	if err != nil {
		if pgerr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgerr.Code {
			case "23505":
				return repository.ErrEntityAlreadyExists
			default:
				return fmt.Errorf(
					"%w: msg - %s, pgcode: %s",
					repository.ErrUnknownDBError, pgerr.Message, pgerr.Code,
				)
			}
		}
		return fmt.Errorf(
			"%w: detail - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return nil
}

func (repo *ChatRepositorySQL) Delete(ctx context.Context, chatID int64) error {
	id, err := repo.queries.DeleteChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(
				"%w: chat with this id doesn't exist",
				repository.ErrEntityNotFound,
			)
		}
		return fmt.Errorf("%w: detail - %v",
			repository.ErrUnknownDBError, err.Error(),
		)
	}
	if id != chatID {
		return fmt.Errorf(
			"%w: chat with this id doesn't exist",
			repository.ErrEntityNotFound,
		)
	}
	return nil
}
