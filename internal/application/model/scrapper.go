package model

import (
	"encoding/json"
	"time"

	"link-tracker/internal/infrastructure/openapi/bot"
)

type Priority int

const (
	PriorityLow    Priority = iota - 1 // -1
	PriorityMedium                     // 0 - default value
	PriorityHigh                       // 1
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "Low"
	case PriorityMedium:
		return "Medium"
	case PriorityHigh:
		return "High"
	default:
		return "Unknwown priority"
	}
}

func (p Priority) ToProtobufPriority() int32 {
	switch p {
	case PriorityMedium:
		return 0
	case PriorityHigh:
		return 1
	case PriorityLow:
		return 2
	default:
		return 0
	}
}

func PriorityFromProtobugPriority(pbPriority int32) Priority {
	switch pbPriority {
	case 0:
		return PriorityMedium
	case 1:
		return PriorityHigh
	case 2:
		return PriorityLow
	default:
		return PriorityMedium // as a default fallback
	}
}

func PriorityFromString(p string) Priority {
	switch p {
	case "Low":
		return PriorityLow
	case "Medium":
		return PriorityMedium
	case "High":
		return PriorityHigh
	default:
		return PriorityMedium // as a default fallback
	}
}

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
	LinkID       int64
	RawURL       string
	UpdatedAt    time.Time
	UpdateName   string
	UpdateAuthor string
	Description  string
	Priority     Priority
	ChatIDs      []int64
}

func (upd LinkUpdatedEvent) Marshal() ([]byte, error) {
	updateDTO := bot.LinkUpdate{
		Description:  upd.Description,
		Id:           upd.LinkID,
		UpdateAuthor: upd.UpdateAuthor,
		Title:        upd.UpdateName,
		TgChatIds:    upd.ChatIDs,
		UpdatedAt:    upd.UpdatedAt.Format(time.RFC3339),
		Url:          upd.RawURL,
		Priority:     bot.LinkUpdatePriority(upd.Priority.String()),
	}
	return json.Marshal(updateDTO)
}

type OutboxLinkUpdateEvent struct {
	ID     int64
	LinkID int64
	Event  LinkUpdatedEvent
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
