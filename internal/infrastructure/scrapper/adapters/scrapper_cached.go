package adapters

import (
	"context"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/cache"
)

type ScrapperCachedAdapter struct {
	logger     *slog.Logger
	next       ports.ScrapperPort
	linksCache cache.LinksCache
}

func NewScrapperCachedAdapter(
	logger *slog.Logger,
	regularPort ports.ScrapperPort,
	linksCache cache.LinksCache,
) ports.ScrapperPort {
	return &ScrapperCachedAdapter{
		logger:     logger,
		next:       regularPort,
		linksCache: linksCache,
	}
}

func (s *ScrapperCachedAdapter) DeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	deletedLink, err := s.next.DeleteLink(ctx, cmd)
	if err != nil {
		return deletedLink, err
	}
	err = s.linksCache.Delete(ctx, cmd.ChatID)
	if err != nil {
		s.logger.Warn("can't invalidate links cache on DeleteLink", "err", err)
	}
	return deletedLink, nil
}

func (s *ScrapperCachedAdapter) GetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	links, hit, err := s.linksCache.Get(ctx, query.ChatID)
	if err != nil {
		s.logger.Warn("can't get links from cache", "err", err)
	}
	if hit {
		s.logger.Info("get links from cache")
		return links, nil
	}
	links, err = s.next.GetTrackedLinks(ctx, query)
	if err != nil {
		return nil, err
	}
	err = s.linksCache.Set(ctx, query.ChatID, links)
	if err != nil {
		s.logger.Warn("can't cache links", "err", err)
	}
	return links, nil
}

func (s *ScrapperCachedAdapter) AddLink(ctx context.Context, cmd model.TrackLinkCmd) (model.TrackedLinkView, error) {
	link, err := s.next.AddLink(ctx, cmd)
	if err != nil {
		return link, err
	}
	err = s.linksCache.Delete(ctx, cmd.ChatID)
	if err != nil {
		s.logger.Warn("can't invalidate links cache on AddLink", "err", err)
	}
	return link, nil
}

func (s *ScrapperCachedAdapter) DeleteTGChat(ctx context.Context, chatID int64) error {
	err := s.linksCache.Delete(ctx, chatID)
	if err != nil {
		s.logger.Warn("can't delete links cache", "err", err)
	}
	return s.next.DeleteTGChat(ctx, chatID)
}

func (s *ScrapperCachedAdapter) RegisterTGChat(ctx context.Context, chatID int64) error {
	return s.next.RegisterTGChat(ctx, chatID)
}
