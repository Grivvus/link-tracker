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
	Title     string      `json:"title"`
	Body      *string     `json:"body"`
	IssueURL  string      `json:"url"`
	CreatedAt time.Time   `json:"created_at"`
	CreatedBy *SimpleUser `json:"user"`
}

type PullRequest struct {
	Title          string      `json:"title"`
	Body           *string     `json:"body"`
	PullRequestURL string      `json:"url"`
	CreatedAt      time.Time   `json:"created_at"`
	CreatedBy      *SimpleUser `json:"user"`
}

type SimpleUser struct {
	Login string `json:"login"`
	URL   string `json:"html_url"`
}
