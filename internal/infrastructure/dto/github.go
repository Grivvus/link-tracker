package dto

import (
	"time"
)

type GithubRepository struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"html_url"`
}

type Issue struct {
	Title     string    `json:"title"`
	Body      *string   `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type PullRequest struct {
	Title     string    `json:"title"`
	Body      *string   `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
