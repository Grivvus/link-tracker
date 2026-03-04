package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/fsm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
)

const (
	defaultMsgText = "Неизвестная команда"
	startMsgText   = "Привет! Это Бот link-tracker, для получения информации о всех командах, используйте /help"
	helpMsgText    = `
start - Выводит приветственное сообщение,
help - Выводит этот текст
	`
)

type BotApplication struct {
	Bot          *bot.Bot
	fsm          *fsm.FSM
	logger       *slog.Logger
	scrapperPort ports.ScrapperPort
}

func New(
	ctx context.Context, cfg infrastructure.BotConfig,
	logger *slog.Logger, sp ports.ScrapperPort,
) (*BotApplication, error) {
	// so i can pass function-pointer as a default handler
	app := &BotApplication{}
	opts := []bot.Option{
		bot.WithDebug(),
		bot.WithDefaultHandler(app.DefaultHandler),
	}
	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return nil, err
	}
	fsm := newFSM()

	app.logger = logger
	app.Bot = b
	app.fsm = fsm
	app.scrapperPort = sp

	appInit(ctx, app)
	return app, nil
}

func (bapp *BotApplication) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Warn("Default handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   defaultMsgText,
	})
	bapp.HelpHandler(ctx, b, update)
}

func (bapp *BotApplication) StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Start handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   startMsgText,
	})
}

func (bapp *BotApplication) HelpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Help handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   helpMsgText,
	})
}

func (bapp *BotApplication) TrackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Track handler was called for", "chat", update.Message.Chat.ID)
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	switch bapp.fsm.Current(userID) {
	case Default:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Default state",
		})
		bapp.fsm.Transition(userID, Start, ctx, b, chatID, userID)
	case Start:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Start state",
		})
		bapp.fsm.Transition(userID, AskLink, ctx, b, chatID, userID)
	case AskLink:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "AskLink state",
		})
		bapp.fsm.Transition(userID, AskTags, ctx, b, chatID, userID)
	case AskTags:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "AskTags state",
		})
		bapp.fsm.Transition(userID, Finish, ctx, b, chatID, userID)
	case Finish:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Finish state",
		})
		bapp.fsm.Transition(userID, Default, ctx, b, chatID, userID)
	default:
		bapp.logger.Error(
			"TrackHandler. Illegal state of fsm",
			"state", bapp.fsm.Current(userID),
			"user", userID,
		)
	}
}

func (bapp *BotApplication) CancelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Cancel handler was called for", "chat", update.Message.Chat.ID)
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	if bapp.fsm.Current(userID) == Default {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Состояние уже сброшено",
		})
	} else {
		bapp.fsm.Transition(userID, Default, ctx, b, chatID, userID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Состояние было сброшено. Можете начать диалог заново.",
		})
	}
}

func (bapp *BotApplication) UntrackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Untrack handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "untrack",
	})
}

func (bapp *BotApplication) ListHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("List handler was called for", "chat", update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "list",
	})
}

func appInit(ctx context.Context, app *BotApplication) {
	app.Bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "start", Description: "Начинает диалог с ботом"},
			{Command: "help", Description: "Показывает этот текст"},
			{Command: "track", Description: "Начать отслеживание ссылки"},
			{Command: "cancel", Description: "Отменить текущее добавление ссылки"},
			{Command: "untrack", Description: "Прекратить отслеживание ссылки"},
			{Command: "list", Description: "Вывести список всех отслеживаемых ссылок"},
		},
	})

	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "start",
		bot.MatchTypeCommand, app.StartHandler,
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "help",
		bot.MatchTypeCommand, app.HelpHandler,
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "track",
		bot.MatchTypeCommand, app.TrackHandler,
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "cancel",
		bot.MatchTypeCommand, app.CancelHandler,
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "untrack",
		bot.MatchTypeCommand, app.UntrackHandler,
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "list",
		bot.MatchTypeCommand, app.ListHandler,
	)
}
