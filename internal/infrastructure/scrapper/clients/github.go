package clients

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
)

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
	url *url.URL,
) (dto.GithubRepository, error) {
	if !strings.Contains(url.Path, "repos") {
		g.logger.Error("url didn't contains 'repos'", "url", url)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: url-path must starts with '/repos'", ports.ErrBadParams,
		)
	}
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		g.logger.Error("error creating request", "url", url)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.logger.Error("error executing request", "url", url, "err", err)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: exect error: %w", ports.ErrBadResponse, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		g.logger.Error(
			"getting error-response",
			"url", url, "status_code", resp.StatusCode,
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
