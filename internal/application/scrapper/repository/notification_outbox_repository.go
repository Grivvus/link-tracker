package repository

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

type NotificationOutboxRepository interface {
	GetPending(ctx context.Context, startsWith int64, limit int32) ([]model.OutboxLinkUpdateEvent, error)
	MarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64, reason string) error
}
