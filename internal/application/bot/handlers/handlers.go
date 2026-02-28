package handlers

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/fsm"
	botfsm "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/fsm"
)

const (
	defaultMsgText = "Неизвестная команда"
	startMsgText   = "Привет! Это Бот link-tracker, для получения информации о всех командах, используйте /help"
	helpMsgText    = `
start - Выводит приветственное сообщение,
help - Выводит этот текст
	`
)

func DefaultHandler(log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Warn("Default handler was called for", "chat", update.Message.Chat.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   defaultMsgText,
		})
		HelpHandler(log)(ctx, b, update)
	}
}

func StartHandler(log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("Start handler was called for", "chat", update.Message.Chat.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   startMsgText,
		})
	}
}

func HelpHandler(log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("Help handler was called for", "chat", update.Message.Chat.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   helpMsgText,
		})
	}
}

func TrackHandler(fsm *fsm.FSM, log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("Track handler was called for", "chat", update.Message.Chat.ID)
		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID
		switch fsm.Current(userID) {
		case botfsm.Default:
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Default state",
			})
			fsm.Transition(userID, botfsm.Start, ctx, b, chatID, userID)
		case botfsm.Start:
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Start state",
			})
			fsm.Transition(userID, botfsm.AskLink, ctx, b, chatID, userID)
		case botfsm.AskLink:
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "AskLink state",
			})
			fsm.Transition(userID, botfsm.AskTags, ctx, b, chatID, userID)
		case botfsm.AskTags:
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "AskTags state",
			})
			fsm.Transition(userID, botfsm.Finish, ctx, b, chatID, userID)
		case botfsm.Finish:
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Finish state",
			})
			fsm.Transition(userID, botfsm.Default, ctx, b, chatID, userID)
		default:
			log.Error(
				"TrackHandler. Illegal state of fsm",
				"state", fsm.Current(userID),
				"user", userID,
			)
		}
	}
}

func CancelHanlder(fsm *fsm.FSM, log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("Cancel handler was called for", "chat", update.Message.Chat.ID)
		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID
		if fsm.Current(userID) == botfsm.Default {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Состояние уже сброшено",
			})
		} else {
			fsm.Transition(userID, botfsm.Default, ctx, b, chatID, userID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Состояние было сброшено. Можете начать диалог заново.",
			})
		}
	}
}

func UntrackHandler(log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("Untrack handler was called for", "chat", update.Message.Chat.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "untrack",
		})
	}
}

func ListHandler(log *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Info("List handler was called for", "chat", update.Message.Chat.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "list",
		})
	}
}
