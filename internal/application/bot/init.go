package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/fsm"
	botfsm "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/fsm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
)

type BotApplication struct {
	Bot    *bot.Bot
	FSM    *fsm.FSM
	Logger *slog.Logger
}

func New(ctx context.Context, cfg infrastructure.BotConfig, logger *slog.Logger) (BotApplication, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(handlers.DefaultHandler(logger)),
	}
	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return BotApplication{}, err
	}
	fsm := botfsm.New()
	app := BotApplication{
		Bot: b, FSM: fsm,
		Logger: logger,
	}
	appInit(ctx, app)
	return app, nil
}

func appInit(ctx context.Context, app BotApplication) {
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
		bot.MatchTypeCommand, handlers.StartHandler(app.Logger),
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "help",
		bot.MatchTypeCommand, handlers.HelpHandler(app.Logger),
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "track",
		bot.MatchTypeCommand, handlers.TrackHandler(app.FSM, app.Logger),
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "cancel",
		bot.MatchTypeCommand, handlers.CancelHanlder(app.FSM, app.Logger),
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "untrack",
		bot.MatchTypeCommand, handlers.UntrackHandler(app.Logger),
	)
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "list",
		bot.MatchTypeCommand, handlers.ListHandler(app.Logger),
	)
}
