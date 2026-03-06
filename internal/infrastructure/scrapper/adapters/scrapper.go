package adapters

import (
	"context"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
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

func (s *ScrapperAdapter) DeleteLinks(
	ctx context.Context, in dto.DeleteLink,
) (dto.DeleteLinksResponse, error) {
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, chatID int64,
) (dto.TrackedLinksResponse, error) {
	if chatID < 0 {
		return nil, ports.ErrBadParams
	}
	links, err := s.trackingRepo.FindByChat(chatID)
	if err != nil {
		return nil, ports.ErrNoChatWithThisID
	}

	ret := make([]dto.TrackedLink, len(links))
	for i, link := range links {
		ret[i] = dto.TrackedLink{
			ID:      link.ID,
			URL:     link.URL.String(),
			Tags:    link.Tags,
			Filters: link.Filters,
		}
	}
	return ret, nil

}

func (s *ScrapperAdapter) AddLinks(
	ctx context.Context, in dto.AddLink,
) (dto.AddLinksResponse, error) {
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	if chatID < 0 {
		return ports.ErrBadParams
	}
	err := s.chatRepo.Delete(chatID)
	if err != nil {
		return ports.ErrNoChatWithThisID
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
		return ports.ErrChatAlreadyExist
	}
	return nil
}
