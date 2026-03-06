package domain

import (
	"net/url"
)

type Tracking struct {
	ID     int64
	ChatID int64
	URL    *url.URL
	Tags   []string
}

func NewTracking(chatID int64, rawURL string, tags []string) (Tracking, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Tracking{}, NewErrBadURL(rawURL)
	}
	if tags == nil {
		tags = []string{}
	}
	return Tracking{
		ID:     -1,
		ChatID: chatID,
		URL:    parsed,
		Tags:   tags,
	}, nil
}
