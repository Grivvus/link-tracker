package repository

type ChatRepository interface {
	Register(chatID int64) error
	Delete(chatID int64) error
}

func NewChatRepository() ChatRepository {}
