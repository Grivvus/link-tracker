package repository

import "context"

type ChatRepository interface {
	Register(ctx context.Context, chatID int64) error
	Delete(ctx context.Context, chatID int64) error
}
