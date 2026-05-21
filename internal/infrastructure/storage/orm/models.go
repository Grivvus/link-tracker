package orm

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
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

type Notification struct {
	ID                 int64              `db:"id"`
	UpdatedAt          pgtype.Timestamptz `db:"updated_at"`
	LinkID             int64              `db:"link_id"`
	FormedNotification []byte             `db:"formed_notification"`
}

func (n Notification) ToModelOutboxLinkEvent() (model.OutboxLinkUpdateEvent, error) {
	var updateEvent bot.LinkUpdate
	err := json.Unmarshal(n.FormedNotification, &updateEvent)
	if err != nil {
		return model.OutboxLinkUpdateEvent{}, fmt.Errorf(
			"%w can't create model from db row - %w",
			repository.ErrBadParams, err,
		)
	}
	updateEventModel := model.LinkUpdatedEvent{
		LinkID:       n.LinkID,
		RawURL:       updateEvent.Url,
		UpdatedAt:    n.UpdatedAt.Time,
		ChatIDs:      updateEvent.TgChatIds,
		UpdateName:   updateEvent.Title,
		UpdateAuthor: updateEvent.UpdateAuthor,
		Description:  updateEvent.Description,
		Priority:     model.PriorityFromString(string(updateEvent.Priority)),
	}
	return model.OutboxLinkUpdateEvent{
		ID:     n.ID,
		LinkID: n.LinkID,
		Event:  updateEventModel,
	}, nil
}
