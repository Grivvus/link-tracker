package adapters

import (
	"context"
	"errors"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

type BotAdapter struct {
	logger *slog.Logger
	client bot.ClientInterface
}

func NewBotAdapter(logger *slog.Logger, client bot.ClientInterface) ports.BotPort {
	return &BotAdapter{
		logger: logger,
		client: client,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, in dto.LinkUpdate,
) error {
	var body = bot.PostUpdatesJSONRequestBody{
		Id:          &in.ID,
		Description: &in.Description,
		TgChatIds:   &in.TGChatIDs,
		Url:         &in.URL,
	}
	resp, err := b.client.PostUpdates(ctx, body)
	if err != nil {
		b.logger.Error(
			"can't make request NotifyAboutUpdatedLinks",
			"err", err,
		)
		return errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return ports.ErrBadParams
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}
