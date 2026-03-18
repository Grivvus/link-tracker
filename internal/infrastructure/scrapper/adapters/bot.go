package adapters

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	bp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BotAdapter struct {
	logger     *slog.Logger
	client     bot.ClientInterface
	grpcClient bp.BotClient
	useGRPC    bool
}

func NewBotAdapter(
	logger *slog.Logger, client bot.ClientInterface,
	grpcClient bp.BotClient, useGRPC bool,
) ports.BotPort {
	return &BotAdapter{
		logger:     logger,
		client:     client,
		grpcClient: grpcClient,
		useGRPC:    useGRPC,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	if b.useGRPC {
		return b.grpcNotify(ctx, event)
	}
	return b.httpNotify(ctx, event)
}

func (b *BotAdapter) httpNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	var body = bot.PostUpdatesJSONRequestBody{
		Id:          event.LinkID,
		Description: event.Description,
		TgChatIds:   event.ChatIDs,
		Url:         event.RawURL,
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
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return ports.ErrBadParams
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (b *BotAdapter) grpcNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	_, err := b.grpcClient.NotifyAboutUpdate(ctx, &bp.LinkUpdate{
		Id:          event.LinkID,
		Url:         event.RawURL,
		Description: event.Description,
		TgChatIds:   event.ChatIDs,
	})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return ports.ErrBadParams
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}
