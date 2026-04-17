package orm

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type NotificationOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationOutboxRepository(pool *pgxpool.Pool) repository.NotificationOutboxRepository {
	return &NotificationOutboxRepository{
		pool: pool,
	}
}

func (repo *NotificationOutboxRepository) GetPending(
	ctx context.Context, startsWith int64, limit int32,
) ([]model.OutboxLinkUpdateEvent, error) {
	notifications, err := queryMany(ctx, repo.pool, dialect.
		Select("id", "updated_at", "link_id", "formed_notification").
		From(goqu.T("notification_event")).
		Where(goqu.Ex{"id": goqu.Op{"gt": startsWith}}).
		Limit(uint(limit)).
		Prepared(true), pgx.RowToStructByName[Notification])
	if err != nil {
		return nil, fmt.Errorf(
			"%w can't fetch pending events - %w",
			repository.ErrUnknownDBError, err,
		)
	}
	outboxEvents := make([]model.OutboxLinkUpdateEvent, len(notifications))
	for i, notification := range notifications {
		outboxEvents[i], err = notification.ToModelOutboxLinkEvent()
		if err != nil {
			return nil, fmt.Errorf(
				"%w can't create model from db row - %w",
				repository.ErrBadParams, err,
			)
		}
	}

	return outboxEvents, nil
}

func (repo *NotificationOutboxRepository) MarkSent(
	ctx context.Context, ids []int64,
) error {
	_, err := withTx(ctx, repo.pool, func(tx pgx.Tx) (int, error) {
		for _, id := range ids {
			err := repo.MarkSentSingle(ctx, id)
			if err != nil {
				return 0, err
			}
		}
		return 0, nil
	})
	return err
}

func (repo *NotificationOutboxRepository) MarkFailed(
	ctx context.Context, id int64, reason string,
) error {
	err := execDataset(ctx, repo.pool, dialect.
		Update(goqu.T("notification_event")).
		Set(goqu.Record{"failing_reason": reason, "failed": true}).
		Where(goqu.Ex{"id": id}).
		Prepared(true),
	)
	if err != nil {
		return fmt.Errorf(
			"%w: can't mark event %d as failed - %w",
			repository.ErrUnknownDBError, id, err,
		)
	}
	return nil
}

func (repo *NotificationOutboxRepository) MarkSentSingle(ctx context.Context, id int64) error {
	err := execDataset(ctx, repo.pool, dialect.
		Update(goqu.T("notification_event")).
		Set(goqu.Record{"sent": true}).
		Where(goqu.Ex{"id": id}).
		Prepared(true),
	)
	if err != nil {
		return fmt.Errorf(
			"%w: can't mark event %d as sent - %w",
			repository.ErrUnknownDBError, id, err,
		)
	}
	return nil
}
