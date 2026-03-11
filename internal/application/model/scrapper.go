package model

import "time"

type TrackLinkCmd struct {
	ChatID int64
	RawURL string
	Tags   []string
}

type UntrackLinkCmd struct {
	ChatID int64
	RawURL string
}

type ListTrackedLinksQuery struct {
	ChatID int64
}

type TrackedLinkView struct {
	LinkID int64
	RawURL string
	Tags   []string
}

type LinkUpdatedEvent struct {
	LinkID      int64
	RawURL      string
	Description string
	ChatIDs     []int64
}

type LinkState struct {
	Fingerprint string
	UpdatedAt   time.Time
	Title       string
}
