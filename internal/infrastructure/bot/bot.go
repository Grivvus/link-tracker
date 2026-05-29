package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/fsm"
	"link-tracker/internal/application/bot/repository"
	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/infrastructure"
	appmetrics "link-tracker/internal/infrastructure/metrics"
)

const (
	defaultMsgText = "Неизвестная команда"
	startMsgText   = "Привет! Это Бот link-tracker, для получения информации о всех командах, используйте /help"
	helpMsgText    = `
start - Выводит приветственное сообщение,
help - Выводит этот текст,
track - Начинает процедуру добавления ссылки для отслеживания,
cancel - Прерывает начатую процедуру добавления,
untrack - Удаляет отслеживаемую ссылку по ее названию,
list - Выводит отслеживаемые ссылки,
		`
	msgChatAlreadyRegistered = "А мы про тебя уже все знаем!"
	msgChatRegisterFailed    = "Не получилось зарегистрировать чат, попробуй позже"
	msgChatRegisteredOK      = "Чат был успешно зарегистрирован"

	msgTrackStartFailed     = "Не могу начать добавление ссылки, попробуйте позже"
	msgTrackAlreadyStarted  = "Неправильное использование команды /track, ввод ссылки уже был начат"
	msgTrackInvalidURL      = "Не получилось добавить ссылку - проверьте корректность ссылки и попробуйте начать сначала"
	msgTrackBadTags         = "Не получилось добавить теги, проверьте формат и повторите их ввод"
	msgTrackInternalFailure = "что-то пошло не так, введите ссылку заново"
	msgLinkAlreadyTracked   = "ссылка уже отслеживается"
	msgTrackSaveFailed      = "не получилось сохранить добавленную ссылку, попробуйте позже"
	msgTrackSaveOK          = "Ссылка была успешно сохранена"

	msgCancelAlreadyReset = "Состояние уже сброшено"
	msgCancelResetOK      = "Состояние было сброшено. Можете начать диалог заново."

	msgUntrackUsage = "Неправильное использование: формат - '/untrack ссылка_для_удаления'"
	msgBadParams    = "Некорректные параметры запроса"
	msgNoLink       = "Такой ссылки не существует"
	msgNoChat       = "Вы еще не зарегистрировались, воспользуйтесь командой /start"
	msgUnexpected   = "Что-то пошло не так, попробуйте повторить свой запрос позже"
	msgUntrackFmt   = "Ссылка '%s' была успешно удалена"

	msgListTooManyFilters = "Множественные фильтры по тегам не поддерживаются, будет использован первый фильтр"
	msgListUnexpected     = "Что-то пошло не так, попробуйте повторить запрос позже"
	msgNoTrackedLinks     = "Нет отслеживаемых ссылок"
	msgNoFilteredLinks    = "Нет ссылок, удовлетворяющих фильтру"
	msgListLineFmt        = "%s tags: %v\r\n"
)

type BotApplication struct {
	Bot           *bot.Bot
	fsm           *fsm.FSM
	dialogStorage repository.DialogDraftsRepository
	logger        *slog.Logger
	scrapperPort  ports.ScrapperPort
	retrier       *retry.Retrier
	metrics       *appmetrics.BotMetrics
}

func New(
	ctx context.Context, cfg infrastructure.Config,
	logger *slog.Logger, sp ports.ScrapperPort,
	rcfg infrastructure.ReliabiltyConfig,
	metrics *appmetrics.BotMetrics,
) (*BotApplication, error) {
	retrier := retry.New(
		retry.Attempts(uint(rcfg.RetryPolicy)),
		retry.Delay(time.Duration(rcfg.RetryPolicyDelayMS)*time.Millisecond),
	)
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
	app.retrier = retrier
	app.metrics = metrics
	return app, nil
}

func (bapp *BotApplication) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Warn("Default handler was called for", "chat", update.Message.Chat.ID)
	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   defaultMsgText,
	})
	bapp.HelpHandler(ctx, b, update)
}

func (bapp *BotApplication) StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	start := time.Now()

	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   startMsgText,
	})
	err := bapp.scrapperPort.RegisterTGChat(ctx, update.Message.Chat.ID)
	if err != nil {
		if errors.Is(err, ports.ErrChatAlreadyExist) {
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   msgChatAlreadyRegistered,
			})
		}
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   msgChatRegisterFailed,
		})
	}
	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   msgChatRegisteredOK,
	})

	const methodName = "start"
	// .Milliseconds() returns int64, so i get .Seconds() * 1000
	duration := time.Since(start).Seconds() * 1000
	bapp.metrics.CommandDurationMSTotal.
		WithLabelValues( /* scope */ "sync", methodName).Observe(duration)
	bapp.metrics.CommandRequestsTotatl.WithLabelValues(methodName).Inc()
}

func (bapp *BotApplication) HelpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	bapp.logger.Info("Help handler was called for", "chat", update.Message.Chat.ID)
	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
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
			bapp.logger.Error(
				"Dialog filling failed",
				"chat", chatID, "error", err,
			)
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgTrackStartFailed,
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
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgTrackAlreadyStarted,
		})
		bapp.fsm.Reset(userID)
	}
}

func (bapp *BotApplication) InputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	start := time.Now()

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
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgTrackInvalidURL,
			})
			bapp.fsm.Reset(userID)
			return
		}
		bapp.fsm.Transition(userID, AskLink, ctx, b, chatID, userID)
	case AskLink:
		tags := update.Message.Text
		var tagsSplited []string
		if tags == "" || tags == "-" {
			tagsSplited = make([]string, 0)
		} else {
			tagsSplited = strings.Split(tags, ",")
		}
		err := bapp.dialogStorage.AddTags(chatID, tagsSplited)
		if err != nil {
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgTrackBadTags,
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
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgTrackInternalFailure,
			})
			bapp.fsm.Reset(userID)
			return
		}
		_, err = bapp.scrapperPort.AddLink(ctx, completedLink)
		if err != nil {
			if errors.Is(err, ports.ErrLinkAlreadyTracked) {
				_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   msgLinkAlreadyTracked,
				})
			}
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgTrackSaveFailed,
			})
			bapp.fsm.Reset(userID)
			return
		}
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgTrackSaveOK,
		})
		bapp.fsm.Transition(userID, Default, ctx, b, chatID, userID)

		const methodName = "track"
		duration := time.Since(start).Seconds() * 1000
		bapp.metrics.CommandDurationMSTotal.
			WithLabelValues( /* scope */ "sync", methodName).Observe(duration)
		bapp.metrics.CommandRequestsTotatl.WithLabelValues(methodName).Inc()
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
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgCancelAlreadyReset,
		})
	} else {
		bapp.fsm.Transition(userID, Default, ctx, b, chatID, userID)
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgCancelResetOK,
		})
	}
}

func (bapp *BotApplication) UntrackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	start := time.Now()
	bapp.logger.Info("Untrack handler was called for", "chat", update.Message.Chat.ID)
	text := update.Message.Text
	textWords := strings.Split(text, " ")
	if len(textWords) != 2 {
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   msgUntrackUsage,
		})
		return
	}
	resp, err := bapp.scrapperPort.DeleteLink(ctx, model.UntrackLinkCmd{
		ChatID: update.Message.Chat.ID, RawURL: textWords[1],
	})
	if err != nil {
		var msgOnErr string
		switch {
		case errors.Is(err, ports.ErrBadParams):
			msgOnErr = msgBadParams
		case errors.Is(err, ports.ErrNoLink):
			msgOnErr = msgNoLink
		case errors.Is(err, ports.ErrNoChatWithThisID):
			msgOnErr = msgNoChat
		default:
			msgOnErr = msgUnexpected
		}

		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID, Text: msgOnErr,
		})
		return
	}
	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf(msgUntrackFmt, resp.RawURL),
	})

	const methodName = "untrack"
	duration := time.Since(start).Seconds() * 1000
	bapp.metrics.CommandDurationMSTotal.
		WithLabelValues( /* scope */ "sync", methodName).Observe(duration)
	bapp.metrics.CommandRequestsTotatl.WithLabelValues(methodName).Inc()
}

func (bapp *BotApplication) ListHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	start := time.Now()

	bapp.logger.Info("List handler was called for", "chat", update.Message.Chat.ID)
	chatID := update.Message.Chat.ID
	var filter string
	words := strings.Split(update.Message.Text, " ")
	if len(words) >= 2 {
		filter = words[1]
		if len(words) > 2 {
			_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msgListTooManyFilters,
			})
		}
	}
	resp, err := bapp.scrapperPort.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{
		ChatID: chatID,
	})
	if err != nil {
		var msgOnErr string
		switch {
		case errors.Is(err, ports.ErrBadParams):
			msgOnErr = msgBadParams
		case errors.Is(err, ports.ErrNoChatWithThisID):
			msgOnErr = msgNoChat
		default:
			msgOnErr = msgListUnexpected
		}
		_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msgOnErr,
		})
		return
	}
	_ = bapp.sendMsgWithRetry(b, ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   buildListAnswer(resp, filter),
	})

	const methodName = "list"
	duration := time.Since(start).Seconds() * 1000
	bapp.metrics.CommandDurationMSTotal.
		WithLabelValues( /* scope */ "sync", methodName).Observe(duration)
	bapp.metrics.CommandRequestsTotatl.WithLabelValues(methodName).Inc()
}

func (bapp *BotApplication) sendMsgWithRetry(
	b *bot.Bot,
	ctx context.Context, params *bot.SendMessageParams,
) error {
	err := bapp.retrier.Do(func() error {
		_, err := b.SendMessage(ctx, params)
		return err
	})
	bapp.logErr(err, params)
	return err
}

func (bapp *BotApplication) logErr(err error, params *bot.SendMessageParams) {
	if err != nil {
		bapp.logger.Error("can't send message to telegram", "err", err, "params", *params)
	}
}

func buildListAnswer(resp []model.TrackedLinkView, tagFilter string) string {
	if len(resp) == 0 {
		return msgNoTrackedLinks
	}
	var sb strings.Builder
	for _, link := range resp {
		if tagFilter != "" && !slices.Contains(link.Tags, tagFilter) {
			continue
		}
		fmt.Fprintf(&sb, msgListLineFmt, link.RawURL, link.Tags)
	}
	msg := sb.String()
	if len(msg) == 0 {
		return msgNoFilteredLinks
	}
	return msg
}

func appInit(ctx context.Context, app *BotApplication) {
	_, _ = app.Bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
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
