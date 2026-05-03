package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
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
}

func NewScrapperPort(
	logger *slog.Logger, client scrapper.ClientInterface,
	grpcClient pb.ScrapperClient, useGRPC bool, callTimeoutSecond int64,
) ports.ScrapperPort {
	return &ScrapperAdapter{
		client:      client,
		log:         logger,
		grpcClient:  grpcClient,
		useGRPC:     useGRPC,
		callTimeout: time.Duration(callTimeoutSecond) * time.Second,
	}
}

func (s *ScrapperAdapter) DeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	if s.useGRPC {
		return s.grpcDeleteLink(tctx, cmd)
	}
	return s.httpDeleteLink(tctx, cmd)
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	if s.useGRPC {
		return s.grpcGetTrackedLinks(tctx, query)
	}
	return s.httpGetTrackedLinks(tctx, query)
}

func (s *ScrapperAdapter) AddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	if s.useGRPC {
		return s.grpcAddLink(tctx, cmd)
	}
	return s.httpAddLink(tctx, cmd)
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	if s.useGRPC {
		return s.grpcDeleteTGChat(tctx, chatID)
	}
	return s.httpDeleteTGChat(tctx, chatID)
}

func (s *ScrapperAdapter) RegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	tctx, cancelFunc := context.WithTimeout(ctx, s.callTimeout)
	defer cancelFunc()
	if s.useGRPC {
		return s.grpcRegisterTGChat(tctx, chatID)
	}
	return s.httpRegisterTGChat(tctx, chatID)
}

func (s *ScrapperAdapter) httpDeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	var params scrapper.DeleteLinksParams
	params.TgChatId = cmd.ChatID
	var body = scrapper.DeleteLinksJSONRequestBody{
		Link: cmd.RawURL,
	}
	resp, err := s.client.DeleteLinks(ctx, &params, body)
	if err != nil {
		s.log.Error(
			"can't make request DeleteLinks",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, fmt.Errorf("can't make a DeleteLinks request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case http.StatusNotFound:
			return model.TrackedLinkView{}, ports.ErrNoLink
		default:
			return model.TrackedLinkView{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"can't decode response from DeleteLinks request",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, errors.New("can't get answer from server")
	}

	out := linkResponseToDTO(respBody)

	return out, nil
}

func (s *ScrapperAdapter) grpcDeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	lresp, err := s.grpcClient.DeleteLinkFromTracking(ctx, &pb.RemoveLinkRequest{
		Id:   &pb.TgChatId{Id: cmd.ChatID},
		Link: cmd.RawURL,
	})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case codes.NotFound:
			return model.TrackedLinkView{}, ports.ErrNoLink
		default:
			return model.TrackedLinkView{}, errors.New("unexpected error")
		}
	}
	return model.TrackedLinkView{
		LinkID: lresp.Id,
		RawURL: lresp.Url,
		Tags:   lresp.Tags,
	}, nil
}

func (s *ScrapperAdapter) httpGetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	params := scrapper.GetLinksParams{
		TgChatId: query.ChatID,
	}
	resp, err := s.client.GetLinks(ctx, &params)
	if err != nil {
		s.log.Error(
			"can't make request GetLinks",
			"err", err,
			"for chat", query.ChatID,
		)
		return nil, fmt.Errorf("can't make a GetTrackedLinks request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return nil, ports.ErrBadParams
		case http.StatusNotFound:
			return nil, ports.ErrNoChatWithThisID
		default:
			return nil, errors.New("unexpected error")
		}
	}

	var respBody scrapper.ListLinksResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"can't decode response from GetLinks",
			"err", err,
			"for chat", query.ChatID,
		)
		return nil, errors.New("unexpected error")
	}

	var out []model.TrackedLinkView
	if respBody.Links == nil {
		s.log.Warn(
			"ListLinksResponse.Links shouldn't be null",
			"for chat", query.ChatID,
		)
		out = make([]model.TrackedLinkView, 0)
	} else {
		links := respBody.Links
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
	links, err := s.grpcClient.GetTrackedLinks(ctx, &pb.TgChatId{Id: query.ChatID})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return nil, ports.ErrBadParams
		case codes.NotFound:
			return nil, ports.ErrNoChatWithThisID
		default:
			return nil, errors.New("unexpected error")
		}
	}
	linksView := make([]model.TrackedLinkView, links.Size)
	for i, link := range links.Links {
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
	resp, err := s.client.PostLinks(ctx, &params, body)
	if err != nil {
		s.log.Error(
			"can't make PostLinks request",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case http.StatusNotFound:
			return model.TrackedLinkView{}, ports.ErrNoChatWithThisID
		case http.StatusConflict:
			return model.TrackedLinkView{}, ports.ErrLinkAlreadyTracked
		default:
			return model.TrackedLinkView{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"ScrapperAdapter, AddLinks, can't decode response",
			"err", err,
			"for chat", cmd.ChatID,
		)
		return model.TrackedLinkView{}, errors.New("unexpected error")
	}

	return linkResponseToDTO(respBody), nil
}

func (s *ScrapperAdapter) grpcAddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	lresp, err := s.grpcClient.AddLinkToTracking(ctx, &pb.AddLinkRequest{
		ChatId: &pb.TgChatId{Id: cmd.ChatID},
		Link:   cmd.RawURL,
		Tags:   cmd.Tags,
	})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case codes.NotFound:
			return model.TrackedLinkView{}, ports.ErrNoChatWithThisID
		case codes.AlreadyExists:
			return model.TrackedLinkView{}, ports.ErrLinkAlreadyTracked
		default:
			return model.TrackedLinkView{}, errors.New("unexpected error")
		}
	}
	return model.TrackedLinkView{
		LinkID: lresp.Id,
		Tags:   lresp.Tags,
		RawURL: lresp.Url,
	}, nil
}

func (s *ScrapperAdapter) httpDeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	resp, err := s.client.DeleteTgChatId(ctx, chatID)
	if err != nil {
		slog.Error(
			"can't make request DeleteTGChat",
			"err", err,
			"for chat", chatID,
		)
		return errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return ports.ErrBadParams
		case http.StatusNotFound:
			return ports.ErrNoChatWithThisID
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (s *ScrapperAdapter) grpcDeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	_, err := s.grpcClient.DeleteChat(ctx, &pb.TgChatId{Id: chatID})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return ports.ErrBadParams
		case codes.NotFound:
			return ports.ErrNoChatWithThisID
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (s *ScrapperAdapter) httpRegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	resp, err := s.client.PostTgChatId(ctx, chatID)
	if err != nil {
		slog.Error(
			"can't make request RegisterTGChat",
			"err", err,
			"for chat", chatID,
		)
		return errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return ports.ErrBadParams
		case http.StatusConflict:
			return ports.ErrChatAlreadyExist
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (s *ScrapperAdapter) grpcRegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	_, err := s.grpcClient.RegisterChat(ctx, &pb.TgChatId{Id: chatID})
	if err != nil {
		statusCode := status.Code(err)
		switch statusCode {
		case codes.InvalidArgument:
			return ports.ErrBadParams
		case codes.AlreadyExists:
			return ports.ErrChatAlreadyExist
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
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
