package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
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
	ctx context.Context, in dto.DeleteLink,
) (dto.DeleteLinkResponse, error) {
	var params scrapper.DeleteLinksParams
	params.TgChatId = in.ChatID
	var body = scrapper.DeleteLinksJSONRequestBody{
		Link: in.Link,
	}
	resp, err := s.client.DeleteLinks(ctx, &params, body)
	if err != nil {
		s.log.Error(
			"can't make request DeleteLinks",
			"err", err,
			"for chat", in.ChatID,
		)
		return dto.DeleteLinkResponse{}, fmt.Errorf("can't make a DeleteLinks request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return dto.DeleteLinkResponse{}, ports.ErrBadParams
		case 404:
			return dto.DeleteLinkResponse{}, ports.ErrNoLink
		default:
			return dto.DeleteLinkResponse{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"can't decode response from DeleteLinks request",
			"err", err,
			"for chat", in.ChatID,
		)
		return dto.DeleteLinkResponse{}, errors.New("can't get answer from server")
	}

	out := linkResponseToDTO(respBody)

	return out, nil
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, chatID int64,
) (dto.TrackedLinksResponse, error) {
	var params scrapper.GetLinksParams
	params.TgChatId = chatID
	resp, err := s.client.GetLinks(ctx, &params)
	if err != nil {
		s.log.Error(
			"can't make request GetLinks",
			"err", err,
			"for chat", chatID,
		)
		return dto.TrackedLinksResponse{}, fmt.Errorf("can't make a GetTrackedLinks request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return dto.TrackedLinksResponse{}, ports.ErrBadParams
		case 404:
			return dto.TrackedLinksResponse{}, ports.ErrNoChatWithThisID
		default:
			return dto.TrackedLinksResponse{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.ListLinksResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"can't decode response from GetLinks",
			"err", err,
			"for chat", chatID,
		)
		return dto.TrackedLinksResponse{}, errors.New("unexpected error")
	}

	var out dto.TrackedLinksResponse
	if respBody.Links == nil {
		s.log.Warn("ListLinksResponse.Links shouldn't be null", "for chat", chatID)
		out = []dto.TrackedLink{}
	} else {
		links := respBody.Links
		out = make([]dto.TrackedLink, 0, len(links))
		for _, link := range links {
			dtoLink := linkResponseToDTO(link)
			out = append(out, dtoLink)
		}
	}
	return out, nil
}

func (s *ScrapperAdapter) AddLink(
	ctx context.Context, in dto.AddLink,
) (dto.AddLinkResponse, error) {
	params := scrapper.PostLinksParams{TgChatId: in.ChatID}
	body := scrapper.PostLinksJSONRequestBody{
		Link: in.Link,
		Tags: in.Tags,
	}
	resp, err := s.client.PostLinks(ctx, &params, body)
	if err != nil {
		s.log.Error(
			"can't make PostLinks request",
			"err", err,
			"for chat", in.ChatID,
		)
		return dto.AddLinkResponse{}, errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return dto.AddLinkResponse{}, ports.ErrBadParams
		case 404:
			return dto.AddLinkResponse{}, ports.ErrNoChatWithThisID
		case 409:
			return dto.AddLinkResponse{}, ports.ErrLinkAlreadyTracked
		default:
			return dto.AddLinkResponse{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"ScrapperAdapter, AddLinks, can't decode response",
			"err", err,
			"for chat", in.ChatID,
		)
		return dto.AddLinkResponse{}, errors.New("unexpected error")
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

func linkResponseToDTO(respBody scrapper.LinkResponse) dto.DeleteLinkResponse {
	var out dto.DeleteLinkResponse
	out.ID = respBody.Id
	out.URL = respBody.Url
	if respBody.Tags == nil {
		out.Tags = []string{}
	} else {
		out.Tags = respBody.Tags
	}
	return out
}
