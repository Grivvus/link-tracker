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
	UpdateInfo string
	UpdatedAt  time.Time
	UpdatedBy  string
	Title      string
}

type StackoverflowNewComment struct {
	// Сообщение об изменениях содержит:
	// • текст темы вопроса
	// • имя пользователя
	// • время создания
	// • превью ответа или комментария (первые 200 символов)
	QuestionName   string
	LeftByUser     string
	CreatedAt      time.Time
	CommentPreview string
}

type StackoverflowNewAnswer struct {
	QuestionName string
	LeftByUser   string
	CreatedAt    time.Time
	// первые 200 символов
	AnswerPreview string
}

type GitHubNewIssue struct {
	// Сообщение об изменениях содержит:
	// • название PR или Issue
	// • имя пользователя
	// • время создания
	// • превью описания (первые 200 символов)
	IssueName               string
	LeftByUser              string
	CreatedAt               time.Time
	IssueDescriptionPreview string
}

type GitHubNewPR struct {
	PRName               string
	LeftByUser           string
	CreatedAt            time.Time
	PRDescriptionPreview string
}
