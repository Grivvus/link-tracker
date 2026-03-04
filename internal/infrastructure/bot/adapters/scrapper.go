package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
)

type ScrapperAdapter struct {
	Client scrapper.ClientInterface
	log    *slog.Logger
}

func (s *ScrapperAdapter) DeleteLinks(
	ctx context.Context, chatID int64, in dto.DeleteLink,
) (dto.DeleteLinksResponse, error) {
	var params *scrapper.DeleteLinksParams
	params.TgChatId = chatID
	var body = scrapper.DeleteLinksJSONRequestBody{
		Link: &in.Link,
	}
	resp, err := s.Client.DeleteLinks(ctx, params, body)
	if err != nil {
		s.log.Error(
			"can't make request DeleteLinks",
			"err", err,
			"for chat", chatID,
		)
		return dto.DeleteLinksResponse{}, fmt.Errorf("can't make a DeleteLinks request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return dto.DeleteLinksResponse{}, errors.New("wrong request parameters")
		case 404:
			return dto.DeleteLinksResponse{}, errors.New("unkown chat or link")
		default:
			return dto.DeleteLinksResponse{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"can't decode response from DeleteLinks request",
			"err", err,
			"for chat", chatID,
		)
		return dto.DeleteLinksResponse{}, errors.New("can't get answer from server")
	}

	out := linkResponseToDTO(respBody)

	return out, nil
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, chatID int64,
) (dto.TrackedLinksResponse, error) {
	var params *scrapper.GetLinksParams
	params.TgChatId = chatID
	resp, err := s.Client.GetLinks(ctx, params)
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
			return dto.TrackedLinksResponse{}, errors.New("wrong request parameters")
		case 404:
			return dto.TrackedLinksResponse{}, errors.New("unkown chat")
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
		out.Links = []dto.TrackedLink{}
	} else {
		links := *respBody.Links
		out.Links = make([]dto.TrackedLink, 0, len(links))
		for _, link := range links {
			dtoLink := linkResponseToDTO(link)
			out.Links = append(out.Links, dtoLink)
		}
	}
	return out, nil
}

func (s *ScrapperAdapter) AddLinks(
	ctx context.Context, chatID int64, in dto.AddLink,
) (dto.AddLinksResponse, error) {
	var params scrapper.PostLinksParams
	var body scrapper.PostLinksJSONRequestBody
	resp, err := s.Client.PostLinks(ctx, &params, body)
	if err != nil {
		s.log.Error(
			"can't make PostLinks request",
			"err", err,
			"for chat", chatID,
		)
		return dto.AddLinksResponse{}, errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return dto.AddLinksResponse{}, errors.New("wrong request parameters")
		case 404:
			return dto.AddLinksResponse{}, errors.New("unkown chat")
		case 409:
			return dto.AddLinksResponse{}, errors.New("link was already tracked")
		default:
			return dto.AddLinksResponse{}, errors.New("unexpected error")
		}
	}

	var respBody scrapper.LinkResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		s.log.Error(
			"ScrapperAdapter, AddLinks, can't decode response",
			"err", err,
			"for chat", chatID,
		)
		return dto.AddLinksResponse{}, errors.New("unexpected error")
	}

	return linkResponseToDTO(respBody), nil
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, in dto.DeleteChat,
) error {
	resp, err := s.Client.DeleteTgChatId(ctx, in.ChatID)
	if err != nil {
		slog.Error(
			"can't make request DeleteTGChat",
			"err", err,
			"for chat", in.ChatID,
		)
		return errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return errors.New("invalid request parameters")
		case 404:
			return errors.New("unkown chat")
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func (s *ScrapperAdapter) RegisterTGChat(
	ctx context.Context, in dto.RegisterChat,
) error {
	resp, err := s.Client.PostTgChatId(ctx, in.ChatID)
	if err != nil {
		slog.Error(
			"can't make request RegisterTGChat",
			"err", err,
			"for chat", in.ChatID,
		)
		return errors.New("unexpected error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case 400:
			return errors.New("invalid request parameters")
		case 409:
			return errors.New("chat was already registered")
		default:
			return errors.New("unexpected error")
		}
	}
	return nil
}

func linkResponseToDTO(respBody scrapper.LinkResponse) dto.DeleteLinksResponse {
	var out dto.DeleteLinksResponse
	if respBody.Id == nil || respBody.Url == nil {
		// assertion
		panic("ID and URL is required")
	}
	out.ID = *respBody.Id
	out.URL = *respBody.Url
	if respBody.Tags == nil {
		out.Tags = []string{}
	} else {
		out.Tags = *respBody.Tags
	}
	if respBody.Filters == nil {
		out.Filters = []string{}
	} else {
		out.Filters = *respBody.Filters
	}
	return out
}
