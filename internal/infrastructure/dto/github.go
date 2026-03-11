package dto

import (
	"time"
)

type GithubRepository struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"html_url"`
}
