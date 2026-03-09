package adapters

import (
	"fmt"
	"log/slog"
	"net/url"

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

func (a *LinkStateAdapter) Fetch(url *url.URL) (*domain.Snapshot, error) {
	switch url.Hostname() {
	case "github.com":
		ghDTO, err := a.ghClient.FetchRepo(url)
		if err != nil {
			return nil, err
		}
		return domain.NewSnapshot(url, ghDTO.UpdatedAt, ghDTO.Name)
	case "stackoverflow.com":
		stoverflowDTO, err := a.stackoverflowClient.FetchQuestion(url)
		if err != nil {
			return nil, err
		}
		return domain.NewSnapshot(url, stoverflowDTO.LastActivityDate, stoverflowDTO.Title)
	default:
		return nil, fmt.Errorf("%w: url: %s", ports.ErrBadURL, url.String())
	}
}
