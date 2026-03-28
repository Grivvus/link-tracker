package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/dto"
)

const githubAPIURL = "https://api.github.com"

type GitHubClient struct {
	logger     *slog.Logger
	httpClient *http.Client
}

func NewGitHubClient(logger *slog.Logger) *GitHubClient {
	return &GitHubClient{
		logger:     logger,
		httpClient: &http.Client{},
	}
}

func (g *GitHubClient) FetchRepo(
	ctx context.Context, info domain.GitHubURLInfo,
) (dto.GithubRepository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildGitHubURL(info), nil)
	if err != nil {
		g.logger.Error("error creating request", "url info", info)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	g.logger.Info("request was built", "url", req.URL.String())
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.logger.Error("error executing request", "url info", info, "err", err)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: exect error: %w", ports.ErrBadResponse, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	g.logger.Info("got response", "code", resp.StatusCode)
	if resp.StatusCode >= http.StatusBadRequest {
		g.logger.Error(
			"getting error-response",
			"url info", info, "status_code", resp.StatusCode,
		)
		return dto.GithubRepository{}, ports.ErrBadResponse
	}

	respDTO := dto.GithubRepository{}
	err = json.NewDecoder(resp.Body).Decode(&respDTO)
	if err != nil {
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: can't decode response", ports.ErrBadResponse,
		)
	}
	return respDTO, nil
}

func buildGitHubURL(info domain.GitHubURLInfo) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s", githubAPIURL, "repos", info.Username, info.RepoName,
	)
}
