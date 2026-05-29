package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/application/scrapper/repository"
	"link-tracker/internal/domain"
)

type ScrapperAdapter struct {
	log              *slog.Logger
	chatRepo         repository.ChatRepository
	subscriptionRepo repository.SubscriptionRepository
}

func NewScrapperAdapter(
	l *slog.Logger,
	chatRepo repository.ChatRepository,
	subscriptionRepo repository.SubscriptionRepository,
) ports.ScrapperPort {
	return &ScrapperAdapter{
		log:              l,
		chatRepo:         chatRepo,
		subscriptionRepo: subscriptionRepo,
	}
}

func (s *ScrapperAdapter) DeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	parsed, err := domain.NewLink(cmd.RawURL)
	if err != nil {
		return model.TrackedLinkView{}, ports.ErrBadURL
	}
	link, err := s.subscriptionRepo.FindLinkByURL(ctx, parsed.URL)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return model.TrackedLinkView{}, fmt.Errorf(
				"%w: you are not subscribed on this link", ports.ErrNoLink,
			)
		}
		return model.TrackedLinkView{}, fmt.Errorf(
			"unknown repository error: %w", err,
		)
	}
	sub, err := s.subscriptionRepo.Unsubscribe(ctx, link, cmd.ChatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return model.TrackedLinkView{}, ports.ErrNoLink
		}
		return model.TrackedLinkView{}, errors.New("unexpected error")
	}

	return model.TrackedLinkView{
		LinkID: link.ID,
		RawURL: link.URL.String(),
		Tags:   sub.Tags,
	}, nil
}

func (s *ScrapperAdapter) GetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	subscriptions, err := s.subscriptionRepo.AllChatSubscriptions(ctx, query.ChatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityNotFound) {
			return nil, ports.ErrNoChatWithThisID
		}
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	ret := make([]model.TrackedLinkView, len(subscriptions))
	for i, sub := range subscriptions {
		subLink, err := s.subscriptionRepo.GetLink(ctx, sub.LinkID)
		if err != nil {
			return nil, fmt.Errorf("unexpected error: %w", err)
		}
		ret[i] = model.TrackedLinkView{
			LinkID: sub.LinkID,
			RawURL: subLink.URL.String(),
			Tags:   sub.Tags,
		}
	}
	return ret, nil

}

func (s *ScrapperAdapter) AddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	link, err := domain.NewLink(cmd.RawURL)
	if err != nil {
		slog.Error("", "err", err)
		// always domain.ErrBadUrl
		return model.TrackedLinkView{}, fmt.Errorf(
			"%w: can't fetch all data from url, may be invalid", ports.ErrBadParams,
		)
	}
	sub, err := s.subscriptionRepo.Subscribe(ctx, link, cmd.ChatID, cmd.Tags)
	if err != nil {
		if errors.Is(err, repository.ErrEntityAlreadyExists) {
			return model.TrackedLinkView{}, ports.ErrLinkAlreadyTracked
		} else if errors.Is(err, repository.ErrEntityNotFound) {
			return model.TrackedLinkView{}, ports.ErrNoChatWithThisID
		}

		return model.TrackedLinkView{}, errors.New("unexpected error")
	}
	return model.TrackedLinkView{
		LinkID: sub.LinkID,
		RawURL: link.URL.String(),
		Tags:   sub.Tags,
	}, nil
}

func (s *ScrapperAdapter) DeleteTGChat(
	ctx context.Context, chatID int64,
) error {
	if chatID < 0 {
		return ports.ErrBadParams
	}
	err := s.chatRepo.Delete(ctx, chatID)
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
	err := s.chatRepo.Register(ctx, chatID)
	if err != nil {
		if errors.Is(err, repository.ErrEntityAlreadyExists) {
			return ports.ErrChatAlreadyExist
		}
		return errors.New("unexpected error")
	}
	return nil
}
