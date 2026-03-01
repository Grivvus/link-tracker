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
	Finish  fsm.StateID = "finish"
)

func newFSM() *fsm.FSM {
	return fsm.New(Default, map[fsm.StateID]fsm.Callback{
		Start:   StartCallback,
		AskLink: AskLinkCallback,
		AskTags: AskTagsCallback,
		Finish:  FinishCallback,
	})
}

func StartCallback(f *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("Start callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Начата процедура добавления ссылки. Для отмены используйте /cancel",
	})
}

func AskLinkCallback(f *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("AskLink callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Ссылка добавлена, теперь вы можете прикрепить теги к отслеживаемой ссылке.",
	})
}

func AskTagsCallback(f *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("AskTags callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Теги добавлены. Ссылка сконфигурирована и добавлена",
	})
}

func FinishCallback(f *fsm.FSM, args ...any) {
	ctx := args[0].(context.Context)
	b := args[1].(*bot.Bot)
	chatID := args[2].(int64)
	userID := args[3].(int64)
	_ = userID
	slog.Info("Finish callback was called for", "chat", chatID)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Ссылка успешно добавлена",
	})
}
