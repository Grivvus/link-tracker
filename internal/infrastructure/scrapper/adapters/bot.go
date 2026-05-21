package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	bp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BotAdapter struct {
	logger               *slog.Logger
	client               bot.ClientInterface
	grpcClient           bp.BotClient
	useGRPC              bool
	callTimeout          time.Duration
	retrier              *retry.Retrier
	shouldRetryHTTP      func(int) bool
	shouldRetryGRPC      func(int) bool
	breaker              *gobreaker.TwoStepCircuitBreaker[struct{}]
	fallbackKafkaAdapter ports.BotPort
}

func NewBotAdapter(
	logger *slog.Logger, client bot.ClientInterface,
	grpcClient bp.BotClient, useGRPC bool,
	rcfg infrastructure.ReliabiltyConfig,
	fallbackKafkaAdapter ports.BotPort,
	shouldRetryHTTP func(int) bool,
	shouldRetryGRPC func(int) bool,
) ports.BotPort {
	retrier := retry.New(
		retry.Attempts(uint(rcfg.RetryPolicy)),
		retry.Delay(time.Duration(rcfg.RetryPolicyDelayMS)*time.Millisecond),
	)
	breaker := infrastructure.NewBreaker(logger, rcfg, "bot-client", func(err error) bool {
		return errors.Is(err, ports.ErrBadParams)
	})
	return &BotAdapter{
		logger:               logger,
		client:               client,
		grpcClient:           grpcClient,
		useGRPC:              useGRPC,
		callTimeout:          time.Duration(rcfg.ExternalServicesTimeoutSecond) * time.Second,
		retrier:              retrier,
		breaker:              breaker,
		fallbackKafkaAdapter: fallbackKafkaAdapter,
		shouldRetryHTTP:      shouldRetryHTTP,
		shouldRetryGRPC:      shouldRetryGRPC,
	}
}

func (b *BotAdapter) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	if b.breaker.State() == gobreaker.StateOpen {
		return b.fallbackKafkaAdapter.NotifyAboutUpdatedLinks(ctx, event)
	}
	tctx, cancelFunc := context.WithTimeout(ctx, b.callTimeout)
	defer cancelFunc()
	_, err := infrastructure.ExecuteWithBreaker(b.breaker, func() (int, error) {
		if b.useGRPC {
			return 0, b.grpcNotify(tctx, event)
		}
		return 0, b.httpNotify(tctx, event)
	})
	return err
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
		Priority:     bot.LinkUpdatePriority(event.Priority.String()),
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
					if b.shouldRetryHTTP(resp.StatusCode) {
						return errors.New("unexpected error")
					}
					return retry.Unrecoverable(errors.New("unexpected error"))
				}
			}
			return nil
		})
}

func (b *BotAdapter) grpcNotify(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	return b.retrier.Do(
		func() error {
			_, err := b.grpcClient.NotifyAboutUpdate(ctx, &bp.LinkUpdate{
				Id:           event.LinkID,
				Url:          event.RawURL,
				Title:        event.UpdateName,
				UpdateAuthor: event.UpdateAuthor,
				UpdatedAt:    event.UpdatedAt.Format(time.RFC3339),
				Description:  event.Description,
				TgChatIds:    event.ChatIDs,
				Priority:     bp.Priority(event.Priority.ToProtobufPriority()),
			})
			if err != nil {
				statusCode := status.Code(err)
				switch statusCode {
				case codes.InvalidArgument:
					return retry.Unrecoverable(ports.ErrBadParams)
				default:
					if b.shouldRetryGRPC(int(statusCode)) {
						return errors.New("unexpected error")
					}
					return retry.Unrecoverable(errors.New("unexpected error"))
				}
			}
			return nil
		})
}
