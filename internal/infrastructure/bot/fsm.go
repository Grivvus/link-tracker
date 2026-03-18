package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/fsm"
)

const (
	Default fsm.StateID = "default"
	Start   fsm.StateID = "start"
	AskLink fsm.StateID = "ask_link"
	AskTags fsm.StateID = "ask_tags"
)

func newFSM() *fsm.FSM {
	return fsm.New(Default, map[fsm.StateID]fsm.Callback{
		Start:   StartCallback,
		AskLink: AskLinkCallback,
		AskTags: AskTagsCallback,
	})
}

func StartCallback(_ *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("Start callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: `
Для отмены используйте /cancel.
Начата процедура добавления ссылки, введите url, который хотите отслеживать.
`,
	})
}

func AskLinkCallback(_ *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("AskLink callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: `
Ссылка добавлена, теперь вы можете прикрепить теги к отслеживаемой ссылке.
Формат: "tag,tag,tag".
Если не хотите добавлять теги, используйте -.
`,
	})
}

func AskTagsCallback(_ *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("AskTags callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Теги добавлены. Ссылка сконфигурирована и добавлена. Что бы подтвердить, введите 'Ok'",
	})
}
