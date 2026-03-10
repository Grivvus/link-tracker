package domain

type Subscription struct {
	ID     int64
	LinkID int64
	Tags   []string
	ChatID int64
}

func NewSubscription(id, linkID, chatID int64, tags []string) Subscription {
	return Subscription{
		ID:     id,
		LinkID: linkID,
		Tags:   tags,
		ChatID: chatID,
	}
}
