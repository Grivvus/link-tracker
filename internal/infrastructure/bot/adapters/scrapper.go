package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ScrapperAdapter struct {
	log         *slog.Logger
	client      scrapper.ClientInterface
	callTimeout time.Duration
	grpcClient  pb.ScrapperClient
	useGRPC     bool
	retrier     *retry.Retrier
	breaker     *gobreaker.TwoStepCircuitBreaker[struct{}]
}

func NewScrapperPort(
	logger *slog.Logger, client scrapper.ClientInterface,
	grpcClient pb.ScrapperClient, useGRPC bool,
	rcfg infrastructure.ReliabiltyConfig,
) ports.ScrapperPort {
	retrier := retry.New(
		retry.Attempts(uint(rcfg.RetryPolicy)),
		retry.Delay(time.Duration(rcfg.RetryPolicyDelayMS)*time.Millisecond),
	)
	breaker := infrastructure.NewBreaker(
		logger, rcfg, "scrapper-client",
		func(err error) bool {
			bp := errors.Is(err, ports.ErrBadParams)
			bu := errors.Is(err, ports.ErrBadURL)
			conflictLink := errors.Is(err, ports.ErrLinkAlreadyTracked)
			conflictChat := errors.Is(err, ports.ErrChatAlreadyExist)
			nl := errors.Is(err, ports.ErrNoLink)
			nc := errors.Is(err, ports.ErrNoChatWithThisID)
			return bp || bu || conflictLink || conflictChat || nl || nc
		},
	)
	return &ScrapperAdapter{
		client:      client,
		log:         logger,
		grpcClient:  grpcClient,
		useGRPC:     useGRPC,
		callTimeout: time.Duration(rcfg.ExternalServicesTimeoutSecond) * time.Second,
		breaker:     breaker,
		retrier:     retrier,
	}
}

func (s *ScrapperAdapter) DeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()

	return infrastructure.ExecuteWithBreaker(s.breaker, func() (model.TrackedLinkView, error) {
		if s.useGRPC {
			return s.grpcDeleteLink(tctx, cmd)
		}
		return s.httpDeleteLink(tctx, cmd)
	})
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	return infrastructure.ExecuteWithBreaker(s.breaker, func() ([]model.TrackedLinkView, error) {
		if s.useGRPC {
			return s.grpcGetTrackedLinks(tctx, query)
		}
		return s.httpGetTrackedLinks(tctx, query)
	})
}

func (s *ScrapperAdapter) AddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	return infrastructure.ExecuteWithBreaker(s.breaker, func() (model.TrackedLinkView, error) {
		if s.useGRPC {
			return s.grpcAddLink(tctx, cmd)
		}
		return s.httpAddLink(tctx, cmd)
	})
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	_, err := infrastructure.ExecuteWithBreaker(s.breaker, func() (int, error) {
		if s.useGRPC {
			return 0, s.grpcDeleteTGChat(tctx, chatID)
		}
		return 0, s.httpDeleteTGChat(tctx, chatID)
	})
	return err
}

func (s *ScrapperAdapter) RegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	_, err := infrastructure.ExecuteWithBreaker(s.breaker, func() (int, error) {
		if s.useGRPC {
			return 0, s.grpcRegisterTGChat(tctx, chatID)
		}
		return 0, s.httpRegisterTGChat(tctx, chatID)
	})
	return err
}

func (s *ScrapperAdapter) httpDeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	var params scrapper.DeleteLinksParams
	params.TgChatId = cmd.ChatID
	var body = scrapper.DeleteLinksJSONRequestBody{
		Link: cmd.RawURL,
	}
	var respBody []byte
	err := s.retrier.Do(
		func() error {
			resp, err := s.client.DeleteLinks(ctx, &params, body)
			if err != nil {
				s.log.Error(
					"can't make request DeleteLinks",
					"err", err,
					"for chat", cmd.ChatID,
				)
				return fmt.Errorf("can't make a request: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= http.StatusBadRequest {
				switch resp.StatusCode {
				case http.StatusBadRequest:
					return retry.Unrecoverable(ports.ErrBadParams)
				case http.StatusNotFound:
					return retry.Unrecoverable(ports.ErrNoLink)
				default:
					return fmt.Errorf("unexpected error: %w", err)
				}
			}
			respBody, err = io.ReadAll(resp.Body)
			return err
		})

	if err != nil {
		return model.TrackedLinkView{}, err
	}

	var resp scrapper.LinkResponse
	err = json.NewDecoder(bytes.NewReader(respBody)).Decode(&resp)
	if err != nil {
		s.log.Error(
			"can't decode response from DeleteLinks request",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, errors.New("can't get answer from server")
	}

	out := linkResponseToDTO(resp)

	return out, nil
}

func (s *ScrapperAdapter) grpcDeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	var linkResp *pb.LinkResponse
	err := s.retrier.Do(
		func() error {
			var err error
			linkResp, err = s.grpcClient.DeleteLinkFromTracking(ctx, &pb.RemoveLinkRequest{
				Id:   &pb.TgChatId{Id: cmd.ChatID},
				Link: cmd.RawURL,
			})
			if err != nil {
				statusCode := status.Code(err)
				switch statusCode {
				case codes.InvalidArgument:
					return retry.Unrecoverable(ports.ErrBadParams)
				case codes.NotFound:
					return retry.Unrecoverable(ports.ErrNoLink)
				default:
					return fmt.Errorf("unexpected error: %w", err)
				}
			}
			return nil
		})
	if err != nil {
		return model.TrackedLinkView{}, err
	}
	return model.TrackedLinkView{
		LinkID: linkResp.Id,
		RawURL: linkResp.Url,
		Tags:   linkResp.Tags,
	}, nil
}

func (s *ScrapperAdapter) httpGetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	params := scrapper.GetLinksParams{
		TgChatId: query.ChatID,
	}
	var respBody []byte
	err := s.retrier.Do(func() error {
		resp, err := s.client.GetLinks(ctx, &params)
		if err != nil {
			s.log.Error(
				"can't make request GetLinks",
				"err", err,
				"for chat", query.ChatID,
			)
			return fmt.Errorf("can't make a request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= http.StatusBadRequest {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				return retry.Unrecoverable(ports.ErrBadParams)
			case http.StatusNotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		respBody, err = io.ReadAll(resp.Body)
		return err
	})

	if err != nil {
		return nil, err
	}

	var resp scrapper.ListLinksResponse
	err = json.NewDecoder(bytes.NewBuffer(respBody)).Decode(&resp)
	if err != nil {
		s.log.Error(
			"can't decode response from GetLinks",
			"err", err,
			"for chat", query.ChatID,
		)
		return nil, errors.New("unexpected error")
	}

	var out []model.TrackedLinkView
	if resp.Links == nil {
		s.log.Warn(
			"ListLinksResponse.Links shouldn't be null",
			"for chat", query.ChatID,
		)
		out = make([]model.TrackedLinkView, 0)
	} else {
		links := resp.Links
		out = make([]model.TrackedLinkView, 0, len(links))
		for _, link := range links {
			dtoLink := linkResponseToDTO(link)
			out = append(out, dtoLink)
		}
	}
	return out, nil
}

func (s *ScrapperAdapter) grpcGetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	var linksResponse *pb.ListLinksResponse
	err := s.retrier.Do(func() error {
		var err error
		linksResponse, err = s.grpcClient.GetTrackedLinks(ctx, &pb.TgChatId{Id: query.ChatID})
		if err != nil {
			statusCode := status.Code(err)
			switch statusCode {
			case codes.InvalidArgument:
				return retry.Unrecoverable(ports.ErrBadParams)
			case codes.NotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	linksView := make([]model.TrackedLinkView, linksResponse.Size)
	for i, link := range linksResponse.Links {
		linksView[i] = model.TrackedLinkView{
			LinkID: link.Id,
			RawURL: link.Url,
			Tags:   link.Tags,
		}
	}
	return linksView, nil
}

func (s *ScrapperAdapter) httpAddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	params := scrapper.PostLinksParams{TgChatId: cmd.ChatID}
	body := scrapper.PostLinksJSONRequestBody{
		Link: cmd.RawURL,
		Tags: cmd.Tags,
	}

	var respBody []byte
	err := s.retrier.Do(func() error {
		resp, err := s.client.PostLinks(ctx, &params, body)
		if err != nil {
			s.log.Error(
				"can't make PostLinks request",
				"err", err,
				"for chat", cmd.ChatID,
			)
			return fmt.Errorf("can't make a request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= http.StatusBadRequest {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				return retry.Unrecoverable(ports.ErrBadParams)
			case http.StatusNotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			case http.StatusConflict:
				return retry.Unrecoverable(ports.ErrLinkAlreadyTracked)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		respBody, err = io.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return model.TrackedLinkView{}, err
	}

	var linkResp scrapper.LinkResponse
	err = json.NewDecoder(bytes.NewReader(respBody)).Decode(&linkResp)
	if err != nil {
		s.log.Error(
			"ScrapperAdapter, AddLinks, can't decode response",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, fmt.Errorf("can't decode response body: %w", err)
	}

	return linkResponseToDTO(linkResp), nil
}

func (s *ScrapperAdapter) grpcAddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	var linkResp *pb.LinkResponse
	err := s.retrier.Do(func() error {
		var err error
		linkResp, err = s.grpcClient.AddLinkToTracking(ctx, &pb.AddLinkRequest{
			ChatId: &pb.TgChatId{Id: cmd.ChatID},
			Link:   cmd.RawURL,
			Tags:   cmd.Tags,
		})
		if err != nil {
			statusCode := status.Code(err)
			switch statusCode {
			case codes.InvalidArgument:
				return retry.Unrecoverable(ports.ErrBadParams)
			case codes.NotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			case codes.AlreadyExists:
				return retry.Unrecoverable(ports.ErrLinkAlreadyTracked)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return model.TrackedLinkView{}, err
	}
	return model.TrackedLinkView{
		LinkID: linkResp.Id,
		Tags:   linkResp.Tags,
		RawURL: linkResp.Url,
	}, nil
}

func (s *ScrapperAdapter) httpDeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	return s.retrier.Do(func() error {
		resp, err := s.client.DeleteTgChatId(ctx, chatID)
		if err != nil {
			slog.Error(
				"can't make request DeleteTGChat",
				"err", err,
				"for chat", chatID,
			)
			return fmt.Errorf("can't make a request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= http.StatusBadRequest {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				return retry.Unrecoverable(ports.ErrBadParams)
			case http.StatusNotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
}

func (s *ScrapperAdapter) grpcDeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	return s.retrier.Do(func() error {
		_, err := s.grpcClient.DeleteChat(ctx, &pb.TgChatId{Id: chatID})
		if err != nil {
			statusCode := status.Code(err)
			switch statusCode {
			case codes.InvalidArgument:
				return retry.Unrecoverable(ports.ErrBadParams)
			case codes.NotFound:
				return retry.Unrecoverable(ports.ErrNoChatWithThisID)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
}

func (s *ScrapperAdapter) httpRegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	return s.retrier.Do(func() error {
		resp, err := s.client.PostTgChatId(ctx, chatID)
		if err != nil {
			slog.Error(
				"can't make request RegisterTGChat",
				"err", err,
				"for chat", chatID,
			)
			return fmt.Errorf("can't make a request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= http.StatusBadRequest {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				return retry.Unrecoverable(ports.ErrBadParams)
			case http.StatusConflict:
				return retry.Unrecoverable(ports.ErrChatAlreadyExist)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
}

func (s *ScrapperAdapter) grpcRegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	return s.retrier.Do(func() error {
		_, err := s.grpcClient.RegisterChat(ctx, &pb.TgChatId{Id: chatID})
		if err != nil {
			statusCode := status.Code(err)
			switch statusCode {
			case codes.InvalidArgument:
				return retry.Unrecoverable(ports.ErrBadParams)
			case codes.AlreadyExists:
				return retry.Unrecoverable(ports.ErrChatAlreadyExist)
			default:
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		return nil
	})
}

func linkResponseToDTO(respBody scrapper.LinkResponse) model.TrackedLinkView {
	var out model.TrackedLinkView
	out.LinkID = respBody.Id
	out.RawURL = respBody.Url
	if respBody.Tags == nil {
		out.Tags = []string{}
	} else {
		out.Tags = respBody.Tags
	}
	return out
}
