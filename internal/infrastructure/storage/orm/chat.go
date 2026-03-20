package orm

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type ChatRepositoryORM struct {
}

func NewChartRepository() repository.ChatRepository {
	return &ChatRepositoryORM{}
}

func (repo *ChatRepositoryORM) Register(ctx context.Context, chatID int64) error {
}

func (repo *ChatRepositoryORM) Delete(ctx context.Context, chatID int64) error {
}
