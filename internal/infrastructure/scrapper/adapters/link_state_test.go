package adapters_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/domain"
	"link-tracker/internal/infrastructure/dto"
	appmetrics "link-tracker/internal/infrastructure/metrics"
	"link-tracker/internal/infrastructure/scrapper/adapters"
)

type GitHubClientMock struct {
	repoAnswer   dto.GithubRepository
	repoError    error
	issuesAnswer []dto.Issue
	issuesError  error
	prsAnswer    []dto.PullRequest
	prsError     error
}

func NewGitHubClientMock(
	repoAnswer dto.GithubRepository,
	issuesAnswer []dto.Issue,
	prsAnswer []dto.PullRequest,
	prsError error,
	issuesError error,
	repoError error,
) *GitHubClientMock {
	return &GitHubClientMock{
		repoAnswer:   repoAnswer,
		repoError:    repoError,
		issuesAnswer: issuesAnswer,
		issuesError:  issuesError,
		prsAnswer:    prsAnswer,
		prsError:     prsError,
	}
}

func (c *GitHubClientMock) FetchRepo(
	context.Context, domain.GitHubURLInfo,
) (dto.GithubRepository, error) {
	return c.repoAnswer, c.repoError
}

func (c *GitHubClientMock) FetchNewIssues(
	context.Context, domain.GitHubURLInfo, time.Time,
) ([]dto.Issue, error) {
	return c.issuesAnswer, c.issuesError
}

func (c *GitHubClientMock) FetchPRs(
	ctx context.Context, info domain.GitHubURLInfo,
) ([]dto.PullRequest, error) {
	return c.prsAnswer, c.prsError
}

type StackoverflowClientMock struct {
	questionResponse dto.StackoverflowQuestion
	questionError    error
	commentsResponse []dto.StackoverflowComment
	commentsError    error
	answerResponse   []dto.StackoverflowAnswer
	answerError      error
}

func NewStackoverflowClientMock(
	questionResponse dto.StackoverflowQuestion,
	questionError error,
	commentsResponse []dto.StackoverflowComment,
	commentsError error,
	answerResponse []dto.StackoverflowAnswer,
	answerError error,
) *StackoverflowClientMock {
	return &StackoverflowClientMock{
		questionResponse: questionResponse,
		questionError:    questionError,
		commentsResponse: commentsResponse,
		commentsError:    commentsError,
		answerResponse:   answerResponse,
		answerError:      answerError,
	}
}

func (c *StackoverflowClientMock) FetchQuestion(
	ctx context.Context, info domain.StackoverflowURLInfo,
) (dto.StackoverflowQuestion, error) {
	return c.questionResponse, c.questionError
}

func (c *StackoverflowClientMock) FetchQuestionAnswers(
	ctx context.Context, questionID int64,
) ([]dto.StackoverflowAnswer, error) {
	return c.answerResponse, c.answerError
}

func (c *StackoverflowClientMock) FetchQuestionComments(
	ctx context.Context, questionID int64,
) ([]dto.StackoverflowComment, error) {
	return c.commentsResponse, c.commentsError
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testScrapperMetrics() *appmetrics.ScrapperMetrics {
	return appmetrics.NewScrapperMetrics(nil)
}

var testGithubUser = dto.SimpleUser{
	Login: "test_gh_user123",
	URL:   "https://github.com/test_user123",
}

var testStackoverflowUser = dto.ShallowUser{
	Username: "test_stackoverflow_user123",
	UserID:   123,
}

func TestClient_FoundUpdate(t *testing.T) {
	testLink, err := domain.NewLink("https://github.com/Grivvus/dotfiles")
	require.NoError(t, err)
	testLink.LastUpdate = time.Now()

	prCreationTime := time.Now().Add(time.Minute)

	prText := "some string about pr 123"
	ghMockAnswer := dto.PullRequest{
		Title:     "Test",
		CreatedAt: prCreationTime,
		Body:      &prText,
		CreatedBy: &testGithubUser,
	}
	ghClient := NewGitHubClientMock(
		dto.GithubRepository{
			Name:      "dotfiles",
			URL:       "https://github.com/Grivvus/dotfiles",
			UpdatedAt: prCreationTime,
		}, nil,
		[]dto.PullRequest{ghMockAnswer},
		nil, nil, nil,
	)

	linkState := adapters.NewLinkStateAdapter(testLogger(), ghClient, nil, testScrapperMetrics())
	updates, err := linkState.Fetch(context.Background(), testLink)
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	for _, linkState := range updates {
		assert.LessOrEqual(t, len(linkState.UpdateInfo), 200)
		assert.Equal(t, prCreationTime, linkState.UpdatedAt)
	}
}

func TestClient_LongResponseBody(t *testing.T) {
	const url = "https://stackoverflow.com/questions/1"
	link, err := domain.NewLink(url)
	require.NoError(t, err)
	link.LastUpdate = time.Now()

	const longBody = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
`

	commentCreationDate := time.Now().Add(time.Minute)
	newComment := dto.StackoverflowComment{
		CommentID:    1,
		CreationDate: commentCreationDate.UnixMilli(),
		Body:         longBody,
		Owner:        &testStackoverflowUser,
	}

	stackoverflowClient := NewStackoverflowClientMock(
		dto.StackoverflowQuestion{
			LastActivityDate: commentCreationDate.UnixMilli(),
			URL:              url,
		}, nil,
		[]dto.StackoverflowComment{newComment}, nil,
		nil, nil,
	)

	adapter := adapters.NewLinkStateAdapter(testLogger(), nil, stackoverflowClient, testScrapperMetrics())
	updates, err := adapter.Fetch(context.Background(), link)
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	for _, linkState := range updates {
		assert.LessOrEqual(t, len(linkState.UpdateInfo), 200)
		assert.Equal(t, commentCreationDate.UnixMilli(), linkState.UpdatedAt.UnixMilli())
	}

}

func TestClient_NoNewEvents(t *testing.T) {
	testLink, err := domain.NewLink("https://github.com/Grivvus/dotfiles")
	require.NoError(t, err)
	testLink.LastUpdate = time.Now()

	prCreationTime := time.Now().Add(-time.Minute)

	prText := "some string about pr 123"
	ghMockAnswer := dto.PullRequest{
		Title:     "Test",
		CreatedAt: prCreationTime,
		Body:      &prText,
		CreatedBy: &testGithubUser,
	}
	ghClient := NewGitHubClientMock(
		dto.GithubRepository{
			Name:      "dotfiles",
			URL:       "https://github.com/Grivvus/dotfiles",
			UpdatedAt: prCreationTime,
		}, nil,
		[]dto.PullRequest{ghMockAnswer},
		nil, nil, nil,
	)

	linkState := adapters.NewLinkStateAdapter(testLogger(), ghClient, nil, testScrapperMetrics())
	updates, err := linkState.Fetch(context.Background(), testLink)
	require.NoError(t, err)
	assert.Len(t, updates, 0)
}

func TestClient_ReturnsError(t *testing.T) {
	testLink, err := domain.NewLink("https://github.com/Grivvus/dotfiles")
	require.NoError(t, err)
	testLink.LastUpdate = time.Now()

	prCreationTime := time.Now().Add(time.Minute)
	ghClient := NewGitHubClientMock(
		dto.GithubRepository{
			Name:      "dotfiles",
			URL:       "https://github.com/Grivvus/dotfiles",
			UpdatedAt: prCreationTime,
		}, nil,
		nil,
		nil, ports.ErrBadResponse, nil,
	)

	linkState := adapters.NewLinkStateAdapter(testLogger(), ghClient, nil, testScrapperMetrics())
	updates, err := linkState.Fetch(context.Background(), testLink)
	require.ErrorIs(t, err, ports.ErrBadResponse)
	assert.Len(t, updates, 0)
}
