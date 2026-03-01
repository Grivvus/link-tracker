package domain

import (
	"net/url"
)

type Tracking struct {
	ChatID int64
	URL    *url.URL
	Tags   []string
}

func NewTracking(rawURL string, tags []string) (Tracking, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Tracking{}, err
	}
	return Tracking{
		URL:  parsed,
		Tags: tags,
	}, nil
}
