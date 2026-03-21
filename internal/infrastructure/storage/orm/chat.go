package orm

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type ChatRepositoryORM struct {
	db dbRunner
}

func NewChatRepository(pool *pgxpool.Pool) repository.ChatRepository {
	return &ChatRepositoryORM{
		db: pool,
	}
}

func (repo *ChatRepositoryORM) Register(ctx context.Context, chatID int64) error {
	err := execDataset(ctx, repo.db, dialect.
		Insert("chat").
		Cols("id").
		Vals(goqu.Vals{chatID}).
		Prepared(true))
	if err != nil {
		return wrapDuplicateError(err)
	}

	return nil
}

func (repo *ChatRepositoryORM) Delete(ctx context.Context, chatID int64) error {
	_, err := queryOne(ctx, repo.db, dialect.
		Delete("chat").
		Where(goqu.Ex{"id": chatID}).
		Returning("id").
		Prepared(true), pgx.RowTo[int64])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf(
				"%w: chat with this id doesn't exist",
				repository.ErrEntityNotFound,
			)
		}
		return wrapUnknownDBError(err)
	}
	return nil
}
