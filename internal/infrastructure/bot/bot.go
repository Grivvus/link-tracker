package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/fsm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
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
	Bot           *bot.Bot
	fsm           *fsm.FSM
	dialogStorage repository.DialogDraftsRepository
	logger        *slog.Logger
	scrapperPort  ports.ScrapperPort
}

func New(
	ctx context.Context, cfg infrastructure.Config,
	logger *slog.Logger, sp ports.ScrapperPort,
) (*BotApplication, error) {
	// so i can pass function-pointer as a default handler
	app := &BotApplication{}
	opts := []bot.Option{
		bot.WithDebug(),
		bot.WithDefaultHandler(app.DefaultHandler),
	}
	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, err
	}
	fsm := newFSM()

	app.logger = logger
	app.Bot = b
	app.fsm = fsm
	app.scrapperPort = sp
	app.dialogStorage = newDialogDraftsStorage(logger)
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
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   startMsgText,
	})
	bapp.logger.Info("Start handler was called for", "chat", update.Message.Chat.ID)
	err := bapp.scrapperPort.RegisterTGChat(ctx, update.Message.Chat.ID)
	if err != nil {
		if errors.Is(err, ports.ErrChatAlreadyExist) {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "А мы про тебя уже все знаем!",
			})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Не получилось зарегистрировать чат, попробуй позже",
			})
		}
		return
	}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Чат был успешно зарегистрирован",
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
		err := bapp.dialogStorage.StartFilling(chatID)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Не могу начать добавление ссылки, попробуйте позже",
			})
			bapp.fsm.Reset(userID)
			return
		}
		bapp.fsm.Transition(userID, Start, ctx, b, chatID, userID)
	default:
		bapp.logger.Error(
			"TrackHandler. Illegal state of fsm",
			"state", bapp.fsm.Current(userID),
			"user", userID,
		)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Неправильное использование команды /track, ввод ссылки уже был начат",
		})
		bapp.fsm.Reset(userID)
	}
}

func (bapp *BotApplication) InputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("InputHandler was called for", "chat", update.Message.Chat.ID)
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	switch bapp.fsm.Current(userID) {
	case Start:
		rawURL := update.Message.Text
		err := bapp.dialogStorage.AddURL(chatID, rawURL)
		if err != nil {
			bapp.logger.Error(
				"Failed to add URL",
				"chat", chatID, "url", rawURL, "err", err,
			)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Не получилсоь добавить ссылку, попробуйте начать сначала",
			})
			bapp.fsm.Reset(userID)
			return
		}
		bapp.fsm.Transition(userID, AskLink, ctx, b, chatID, userID)
	case AskLink:
		tags := update.Message.Text
		var tagsSplited []string
		if tags == "" {
			tagsSplited = make([]string, 0)
		} else {
			tagsSplited = strings.Split(tags, ",")
		}
		err := bapp.dialogStorage.AddTags(chatID, tagsSplited)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Не получилось добавить теги, проверьте формат и повторите их ввод",
			})
			return
		}
		bapp.fsm.Transition(userID, AskTags, ctx, b, chatID, userID)
	case AskTags:
		completedLink, err := bapp.dialogStorage.GetFilledTracking(chatID)
		if err != nil {
			bapp.logger.Error(
				"Failed to get filled tracking",
				"chat", chatID, "err", err,
			)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "что-то пошло не так, введите ссылку заново",
			})
			bapp.fsm.Reset(userID)
			return
		}
		_, err = bapp.scrapperPort.AddLink(ctx, dto.AddLink{
			ChatID: chatID,
			Link:   completedLink.URL.String(),
			Tags:   completedLink.Tags,
		})
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "не получилось сохранить добавленную ссылку, попробуйте позже",
			})
			bapp.fsm.Reset(userID)
			return
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ссылка была успешно сохранена",
		})
		bapp.fsm.Transition(userID, Default, ctx, b, chatID, userID)
	default:
		bapp.logger.Error(
			"TrackHandler. Illegal state of fsm",
			"state", bapp.fsm.Current(userID),
			"user", userID,
		)
		bapp.DefaultHandler(ctx, b, update)
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
	text := update.Message.Text
	textWords := strings.Split(text, " ")
	if len(textWords) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Неправильное использование: формат - '/untrack ссылка_для_удаления'",
		})
		return
	}
	resp, err := bapp.scrapperPort.DeleteLink(ctx, dto.DeleteLink{
		ChatID: update.Message.Chat.ID, Link: textWords[1],
	})
	if err != nil {
		var msgOnErr string
		if errors.Is(err, ports.ErrBadParams) {
			msgOnErr = "Некорректные параметры запроса"
		} else if errors.Is(err, ports.ErrNoLink) {
			msgOnErr = "Такой ссылки не существует"
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			msgOnErr = "Вы еще не зарегистрировались, воспользуйтесь командой /start"
		} else {
			msgOnErr = "Что-то пошло не так, попробуйте повторить свой запрос позже"
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID, Text: msgOnErr,
		})
		return
	}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Ссылка '%s' была успешно удалена", resp.URL),
	})
}

func (bapp *BotApplication) ListHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("List handler was called for", "chat", update.Message.Chat.ID)
	chatID := update.Message.Chat.ID
	resp, err := bapp.scrapperPort.GetTrackedLinks(ctx, chatID)
	if err != nil {
		var msgOnErr string
		if errors.Is(err, ports.ErrBadParams) {
			msgOnErr = "Некорректные параметры запроса"
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			msgOnErr = "Вы еще не зарегистрировались, воспользуйтесь командой /start"
		} else {
			msgOnErr = "Что-то пошло не так, попробуйте повторить запрос позже"
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgOnErr,
		})
		return
	}
	var sb strings.Builder
	for _, link := range resp {
		sb.WriteString(fmt.Sprintf("%s, tags: %v\n", link.URL, link.Tags))
	}
	if len(resp) == 0 {
		sb.WriteString("No links yet\n")
	}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
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
	app.Bot.RegisterHandler(
		bot.HandlerTypeMessageText, "",
		bot.MatchTypeContains, app.InputHandler,
	)
}
