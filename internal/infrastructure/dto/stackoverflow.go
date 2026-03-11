package dto

import "time"

type StackoverflowQuestion struct {
	LastActivityDate time.Time `json:"last_activity_date"`
	URL              string    `json:"link"`
	Title            string    `json:"title"`
}
