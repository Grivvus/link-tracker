package tests

import "time"

type linkResponse struct {
	ID   int64    `json:"id"`
	URL  string   `json:"url"`
	Tags []string `json:"tags"`
}

type listLinksResponse struct {
	Links []linkResponse `json:"links"`
	Size  int32          `json:"size"`
}

var chatIDCounter int64 = time.Now().UnixNano()
