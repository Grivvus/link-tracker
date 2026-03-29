package dto

type StackoverflowQuestion struct {
	LastActivityDate int64  `json:"last_activity_date"`
	URL              string `json:"link"`
	Title            string `json:"title"`
}

type StackoverflowComment struct {
	CommentID    int64        `json:"comment_id"`
	CreationDate int64        `json:"creation_date"`
	Body         string       `json:"body"`
	Owner        *ShallowUser `json:"owner"`
}

type StackoverflowComments struct {
	Items []StackoverflowComment `json:"items"`
}

type StackoverflowAnswer struct {
	AnswerID         int64        `json:"answer_id"`
	CreationDate     int64        `json:"creation_date"`
	Body             string       `json:"body"`
	LastActivityDate int64        `json:"last_activity_date"`
	Owner            *ShallowUser `json:"owner"`
}

type StackoverflowAnswers struct {
	Items []StackoverflowAnswer `json:"items"`
}

type ShallowUser struct {
	Username string `json:"display_name"`
	UserID   int64  `json:"user_id"`
}
