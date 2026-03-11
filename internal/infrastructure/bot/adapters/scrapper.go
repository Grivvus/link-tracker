package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
)

type ScrapperAdapter struct {
	client scrapper.ClientInterface
	log    *slog.Logger
}

func NewScrapperPort(client scrapper.ClientInterface, logger *slog.Logger) ports.ScrapperPort {
	return &ScrapperAdapter{
		client: client,
		log:    logger,
	}
}

func (s *ScrapperAdapter) DeleteLink(
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
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case 404:
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

func (s *ScrapperAdapter) GetTrackedLinks(
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
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return nil, ports.ErrBadParams
		case 404:
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

func (s *ScrapperAdapter) AddLink(
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
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return model.TrackedLinkView{}, ports.ErrBadParams
		case 404:
			return model.TrackedLinkView{}, ports.ErrNoChatWithThisID
		case 409:
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

func (s *ScrapperAdapter) DeleteTGChat(
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
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return ports.ErrBadParams
		case 404:
			return ports.ErrNoChatWithThisID
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (s *ScrapperAdapter) RegisterTGChat(
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
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return ports.ErrBadParams
		case 409:
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
