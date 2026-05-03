package clients

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/dto"
)

const githubAPIURL = "https://api.github.com"
const githubAPIVersion = "2026-03-10"
const baseQueryParameters = "sort=created&direction=desc&state=open"

type GitHubClient struct {
	logger     *slog.Logger
	httpClient *http.Client
}

func NewGitHubClient(
	logger *slog.Logger,
	rcfg infrastructure.ReliabiltyConfig,
) *GitHubClient {
	return &GitHubClient{
		logger: logger,
		httpClient: &http.Client{
			Timeout: time.Duration(rcfg.ExternalServicesTimeoutSecond) * time.Second,
		},
	}
}

func (g *GitHubClient) FetchRepo(
	ctx context.Context, info domain.GitHubURLInfo,
) (dto.GithubRepository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildGitHubURL(info), nil)
	if err != nil {
		g.logger.Error("error creating request", "err", err)
		return dto.GithubRepository{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", githubAPIVersion)
	return doRequestAndMapResult[dto.GithubRepository](g.logger, g.httpClient, req)
}

func (g *GitHubClient) FetchNewIssues(
	ctx context.Context, info domain.GitHubURLInfo, since time.Time,
) ([]dto.Issue, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		buildGitHubURLQueryNewIssues(info, since), nil,
	)
	if err != nil {
		g.logger.Error("error creating request", "err", err)
		return nil, fmt.Errorf("%w: %w", ports.ErrBadParams, err)
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", githubAPIVersion)
	return doRequestAndMapResult[[]dto.Issue](g.logger, g.httpClient, req)
}

func (g *GitHubClient) FetchPRs(
	ctx context.Context, info domain.GitHubURLInfo,
) ([]dto.PullRequest, error) {
	// 30 latest PR's
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		buildGitHubURLQueryPRs(info), nil,
	)
	if err != nil {
		g.logger.Error("error creating request", "err", err)
		return nil, fmt.Errorf("%w: %w", ports.ErrBadParams, err)
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", githubAPIVersion)
	return doRequestAndMapResult[[]dto.PullRequest](g.logger, g.httpClient, req)
}

func buildGitHubURL(info domain.GitHubURLInfo) string {
	return fmt.Sprintf(
		"%s/repos/%s/%s", githubAPIURL, info.Username, info.RepoName,
	)
}

func buildGitHubURLQueryNewIssues(
	info domain.GitHubURLInfo, since time.Time,
) string {
	return fmt.Sprintf(
		"%s/issues?%s&since=%s",
		buildGitHubURL(info), baseQueryParameters,
		since.Format(time.RFC3339),
	)
}

func buildGitHubURLQueryPRs(info domain.GitHubURLInfo) string {
	return fmt.Sprintf("%s/pulls?%s", buildGitHubURL(info), baseQueryParameters)
}
