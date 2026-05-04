package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	bp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BotAdapter struct {
	logger      *slog.Logger
	client      bot.ClientInterface
	grpcClient  bp.BotClient
	useGRPC     bool
	callTimeout time.Duration
	retrier     *retry.Retrier
}

func NewBotAdapter(
	logger *slog.Logger, client bot.ClientInterface,
	grpcClient bp.BotClient, useGRPC bool,
	callTimeoutSecond int64,
	retrier *retry.Retrier,
) ports.BotPort {
	return &BotAdapter{
		logger:      logger,
		client:      client,
		grpcClient:  grpcClient,
		useGRPC:     useGRPC,
		callTimeout: time.Duration(callTimeoutSecond) * time.Second,
		retrier:     retrier,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	tctx, cancelFunc := context.WithTimeout(ctx, b.callTimeout)
	defer cancelFunc()
	if b.useGRPC {
		return b.grpcNotify(tctx, event)
	}
	return b.httpNotify(tctx, event)
}

func (b *BotAdapter) httpNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	var body = bot.PostUpdatesJSONRequestBody{
		Id:           event.LinkID,
		Description:  event.Description,
		TgChatIds:    event.ChatIDs,
		Url:          event.RawURL,
		Title:        event.UpdateName,
		UpdateAuthor: event.UpdateAuthor,
		UpdatedAt:    event.UpdatedAt.Format(time.RFC3339),
	}
	return b.retrier.Do(
		func() error {
			resp, err := b.client.PostUpdates(ctx, body)
			if err != nil {
				b.logger.Error(
					"can't make request NotifyAboutUpdatedLinks",
					"err", err,
				)
				return fmt.Errorf("can't make a request, err: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= http.StatusBadRequest {
				switch resp.StatusCode {
				case http.StatusBadRequest:
					return retry.Unrecoverable(ports.ErrBadParams)
				default:
					return errors.New("unexpected error")
				}
			}
			return nil
		})
}

func (b *BotAdapter) grpcNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	_, err := b.grpcClient.NotifyAboutUpdate(ctx, &bp.LinkUpdate{
		Id:           event.LinkID,
		Url:          event.RawURL,
		Title:        event.UpdateName,
		UpdateAuthor: event.UpdateAuthor,
		UpdatedAt:    event.UpdatedAt.Format(time.RFC3339),
		Description:  event.Description,
		TgChatIds:    event.ChatIDs,
	})
	return b.retrier.Do(
		func() error {
			if err != nil {
				statusCode := status.Code(err)
				switch statusCode {
				case codes.InvalidArgument:
					return retry.Unrecoverable(ports.ErrBadParams)
				default:
					return errors.New("unexpected error")
				}
			}
			return nil
		})
}
