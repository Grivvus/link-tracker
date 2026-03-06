package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type ScrapperAdapter struct {
	log          *slog.Logger
	chatRepo     repository.ChatRepository
	trackingRepo repository.TrackingRepository
}

func NewScrapperAdapter(
	l *slog.Logger,
	chatRepo repository.ChatRepository,
	trackingRepo repository.TrackingRepository,
) ports.ScrapperPort {
	return &ScrapperAdapter{
		log:          l,
		chatRepo:     chatRepo,
		trackingRepo: trackingRepo,
	}
}

func (s *ScrapperAdapter) DeleteLink(
	ctx context.Context, in dto.DeleteLink,
) (dto.DeleteLinkResponse, error) {
	parsedURL, err := url.Parse(in.Link)
	if err != nil {
		return dto.DeleteLinkResponse{}, fmt.Errorf("%w: link is a not valid url", ports.ErrBadParams)
	}
	removed, err := s.trackingRepo.RemoveByURL(ctx, in.ChatID, parsedURL)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return dto.DeleteLinkResponse{}, ports.ErrNoLink
		}
		return dto.DeleteLinkResponse{}, errors.New("unexpected error")
	}

	return dto.DeleteLinkResponse{
		ID:   removed.ID,
		URL:  removed.URL.String(),
		Tags: removed.Tags,
	}, nil
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, chatID int64,
) (dto.TrackedLinksResponse, error) {
	if chatID < 0 {
		return nil, ports.ErrBadParams
	}
	links, err := s.trackingRepo.FindByChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return nil, ports.ErrNoChatWithThisID
		}
		return nil, errors.New("unexpected error")
	}

	ret := make([]dto.TrackedLink, len(links))
	for i, link := range links {
		ret[i] = dto.TrackedLink{
			ID:   link.ID,
			URL:  link.URL.String(),
			Tags: link.Tags,
		}
	}
	return ret, nil

}

func (s *ScrapperAdapter) AddLink(
	ctx context.Context, in dto.AddLink,
) (dto.AddLinkResponse, error) {
	savableLink, err := domain.NewTracking(in.ChatID, in.Link, in.Tags)
	if err != nil {
		return dto.AddLinkResponse{}, fmt.Errorf(
			"%w: link isn't valid url", ports.ErrBadParams,
		)
	}
	saved, err := s.trackingRepo.Save(ctx, savableLink)
	if err != nil {
		if errors.Is(err, repository.ErrEntityAlreadyExists) {
			return dto.AddLinkResponse{}, ports.ErrLinkAlreadyTracked
		}

		return dto.AddLinkResponse{}, errors.New("unexpected error")
	}
	return dto.AddLinkResponse{
		ID:   saved.ID,
		URL:  saved.URL.String(),
		Tags: saved.Tags,
	}, nil
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	if chatID < 0 {
		return ports.ErrBadParams
	}
	err := s.chatRepo.Delete(chatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return ports.ErrNoChatWithThisID
		}
		return errors.New("unexpected error")
	}
	return nil
}

func (s *ScrapperAdapter) RegisterTGChat(
	ctx context.Context, chatID int64,
) error {
	if chatID < 0 {
		return ports.ErrBadParams
	}
	err := s.chatRepo.Register(chatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityAlreadyExists) {
			return ports.ErrChatAlreadyExist
		}
		return errors.New("unexpected error")
	}
	return nil
}
