package domain

import (
	"net/url"
	"time"
)

type Snapshot struct {
	URL       *url.URL
	UpdatedAt time.Time
	Title     string
	Hash      int64
}

func NewSnapshot(
	url *url.URL, updatedAt time.Time, title string,
) (*Snapshot, error) {
	return &Snapshot{
		URL:       url,
		UpdatedAt: updatedAt,
		Title:     title,
		Hash:      -1,
	}, nil
}
