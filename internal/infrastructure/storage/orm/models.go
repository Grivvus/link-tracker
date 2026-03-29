package orm

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type Link struct {
	ID         int64              `db:"id"`
	URL        string             `db:"url"`
	LastUpdate pgtype.Timestamptz `db:"last_update"`
}

type Subscription struct {
	ID     int64 `db:"id"`
	LinkID int64 `db:"link_id"`
	ChatID int64 `db:"chat_id"`
}

type SubscriptionWithTag struct {
	ID      int64   `db:"subscription_id"`
	LinkID  int64   `db:"link_id"`
	ChatID  int64   `db:"chat_id"`
	TagName *string `db:"tag_name"`
}

type Tag struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type SubscriptionTag struct {
	SubscriptionID int64 `db:"subscription_id"`
	TagID          int64 `db:"tag_id"`
}
