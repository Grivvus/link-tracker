package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/clients"
)

type LinkStateAdapter struct {
	logger              *slog.Logger
	ghClient            *clients.GitHubClient
	stackoverflowClient *clients.StackoverflowClient
}

func NewLinkStateAdapter(logger *slog.Logger) ports.LinkStatePort {
	return &LinkStateAdapter{
		logger:              logger,
		ghClient:            clients.NewGitHubClient(logger),
		stackoverflowClient: clients.NewStackoverflowClient(logger),
	}
}

func (a *LinkStateAdapter) Fetch(ctx context.Context, link domain.Link) (model.LinkState, error) {
	switch link.URL.Hostname() {
	case "github.com":
		isGH, ghinfo := link.GitHubURLInfo()
		if !isGH {
			return model.LinkState{}, fmt.Errorf(
				"%w: exected valid GitHubURLInfo", ports.ErrBadParams,
			)
		}
		ghDTO, err := a.ghClient.FetchRepo(ctx, *ghinfo)
		if err != nil {
			return model.LinkState{}, err
		}
		return model.LinkState{
			Fingerprint: "abcd",
			UpdatedAt:   ghDTO.UpdatedAt,
			Title:       ghDTO.Name,
		}, nil
	case "stackoverflow.com":
		isSt, stinfo := link.StackoverflowURLInfo()
		if !isSt {
			return model.LinkState{}, fmt.Errorf(
				"%w: expected valid StackoverflowURLInfo",
				ports.ErrBadParams,
			)
		}
		stoverflowDTO, err := a.stackoverflowClient.FetchQuestion(ctx, *stinfo)
		if err != nil {
			return model.LinkState{}, err
		}
		return model.LinkState{
			Fingerprint: "abcd",
			UpdatedAt:   stoverflowDTO.LastActivityDate,
			Title:       stoverflowDTO.Title,
		}, nil
	default:
		return model.LinkState{}, fmt.Errorf("%w: url: %s", ports.ErrBadURL, link.URL.String())
	}
}
