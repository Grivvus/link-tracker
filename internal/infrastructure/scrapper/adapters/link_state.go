package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/dto"
)

type GitHubClient interface {
	FetchRepo(context.Context, domain.GitHubURLInfo) (dto.GithubRepository, error)
	FetchNewIssues(context.Context, domain.GitHubURLInfo, time.Time) ([]dto.Issue, error)
	FetchPRs(context.Context, domain.GitHubURLInfo) ([]dto.PullRequest, error)
}

type StackoverflowClient interface {
	FetchQuestion(context.Context, domain.StackoverflowURLInfo) (dto.StackoverflowQuestion, error)
	FetchQuestionAnswers(context.Context, int64) ([]dto.StackoverflowAnswer, error)
	FetchQuestionComments(context.Context, int64) ([]dto.StackoverflowComment, error)
}

type LinkStateAdapter struct {
	logger              *slog.Logger
	ghClient            GitHubClient
	stackoverflowClient StackoverflowClient
}

func NewLinkStateAdapter(
	logger *slog.Logger, ghClient GitHubClient,
	stackoverflowClient StackoverflowClient,
) ports.LinkStatePort {
	return &LinkStateAdapter{
		logger:              logger,
		ghClient:            ghClient,
		stackoverflowClient: stackoverflowClient,
	}
}

func (a *LinkStateAdapter) Fetch(ctx context.Context, link domain.Link) ([]model.LinkState, error) {
	switch link.URL.Hostname() {
	case "github.com":
		return a.fetchGH(ctx, link)
	case "stackoverflow.com":
		return a.fetchStackoverflow(ctx, link)
	default:
		return nil, fmt.Errorf("%w: url: %s", ports.ErrBadURL, link.URL.String())
	}
}

func (a *LinkStateAdapter) fetchGH(ctx context.Context, link domain.Link) ([]model.LinkState, error) {
	isGH, ghinfo := link.GitHubURLInfo()
	if !isGH {
		return nil, fmt.Errorf(
			"%w: expected valid GitHubURLInfo", ports.ErrBadParams,
		)
	}
	ghDTO, err := a.ghClient.FetchRepo(ctx, *ghinfo)
	if err != nil {
		return nil, err
	}
	if !ghDTO.UpdatedAt.After(link.LastUpdate) {
		return []model.LinkState{}, nil
	}
	updates := make([]model.LinkState, 0)
	prs, err := a.ghClient.FetchPRs(ctx, *ghinfo)
	if err != nil {
		return nil, fmt.Errorf("%w: fetching PRs: %v", ports.ErrBadResponse, err)
	}
	for _, pr := range prs {
		if !pr.CreatedAt.After(link.LastUpdate) {
			break
		}
		if pr.Body == nil {
			pr.Body = new(string)
		}

		prNewUpdate := model.LinkState{
			UpdateInfo: reduceBody("New PR: " + *pr.Body),
			UpdatedAt:  pr.CreatedAt,
			Title:      pr.Title,
		}
		if pr.CreatedBy == nil {
			prNewUpdate.UpdatedBy = "Unknown"
		} else {
			prNewUpdate.UpdatedBy = pr.CreatedBy.Login
		}
		updates = append(updates, prNewUpdate)
	}
	newIssues, err := a.ghClient.FetchNewIssues(ctx, *ghinfo, link.LastUpdate)
	if err != nil {
		return nil, fmt.Errorf("%w: fetching issues: %v", ports.ErrBadResponse, err)
	}
	for _, issue := range newIssues {
		if issue.Body == nil {
			issue.Body = new(string)
		}
		issueNewUpdate := model.LinkState{
			Title:      issue.Title,
			UpdatedAt:  issue.CreatedAt,
			UpdateInfo: reduceBody("New Issue: " + *issue.Body),
		}
		if issue.CreatedBy == nil {
			issueNewUpdate.UpdatedBy = "Unknown"
		} else {
			issueNewUpdate.UpdatedBy = issue.CreatedBy.Login
		}
		updates = append(updates, issueNewUpdate)
	}
	return updates, nil
}

func (a *LinkStateAdapter) fetchStackoverflow(
	ctx context.Context, link domain.Link,
) ([]model.LinkState, error) {
	isSt, stinfo := link.StackoverflowURLInfo()
	if !isSt {
		return nil, fmt.Errorf(
			"%w: expected valid StackoverflowURLInfo",
			ports.ErrBadParams,
		)
	}
	stoverflowDTO, err := a.stackoverflowClient.FetchQuestion(ctx, *stinfo)
	if err != nil {
		return nil, err
	}
	lastActivityDate := time.UnixMilli(stoverflowDTO.LastActivityDate)
	if !lastActivityDate.After(link.LastUpdate) {
		// no update since last check
		// zero value for UpdatedAt is fine
		// it will be checked at the next step in LinkPollingUseCase
		return []model.LinkState{}, nil
	}
	newComments, err := a.newCommentsSince(
		ctx, stinfo.QuestionID, stoverflowDTO.Title, link.LastUpdate,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: can't fetch new comments. Initail err: %w",
			ports.ErrBadResponse, err,
		)
	}
	newAnswers, err := a.newAnswersSince(
		ctx, stinfo.QuestionID, stoverflowDTO.Title, link.LastUpdate,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: can't fetch new answers. Initail err: %w",
			ports.ErrBadResponse, err,
		)
	}
	updates := make([]model.LinkState, 0, len(newComments)+len(newAnswers))
	for _, comment := range newComments {
		updates = append(updates, model.LinkState{
			UpdateInfo: comment.CommentPreview,
			UpdatedAt:  comment.CreatedAt,
			Title:      comment.QuestionName,
			UpdatedBy:  comment.LeftByUser,
		})
	}
	for _, answer := range newAnswers {
		updates = append(updates, model.LinkState{
			UpdateInfo: answer.AnswerPreview,
			UpdatedAt:  answer.CreatedAt,
			Title:      answer.QuestionName,
			UpdatedBy:  answer.LeftByUser,
		})
	}
	return updates, nil
}

func (a *LinkStateAdapter) newCommentsSince(
	ctx context.Context, questionID int64,
	questionTitle string, since time.Time,
) ([]model.StackoverflowNewComment, error) {
	comments, err := a.stackoverflowClient.FetchQuestionComments(ctx, questionID)
	if err != nil {
		return nil, err
	}
	newComments := make([]model.StackoverflowNewComment, 0)
	for _, comment := range comments {
		createdAt := time.UnixMilli(comment.CreationDate)
		if !createdAt.After(since) {
			break
		}
		newComments = append(newComments, model.StackoverflowNewComment{
			QuestionName:   questionTitle,
			CreatedAt:      createdAt,
			LeftByUser:     checkAndReturnUsername(comment.Owner),
			CommentPreview: reduceBody(comment.Body),
		})
	}
	return newComments, nil
}

func (a *LinkStateAdapter) newAnswersSince(
	ctx context.Context, questionID int64,
	questionTitle string, since time.Time,
) ([]model.StackoverflowNewAnswer, error) {
	answers, err := a.stackoverflowClient.FetchQuestionAnswers(ctx, questionID)
	if err != nil {
		return nil, err
	}
	newAnswers := make([]model.StackoverflowNewAnswer, 0)
	for _, answer := range answers {
		createdAt := time.UnixMilli(answer.CreationDate)
		if !createdAt.After(since) {
			break
		}
		newAnswers = append(newAnswers, model.StackoverflowNewAnswer{
			QuestionName:  questionTitle,
			LeftByUser:    checkAndReturnUsername(answer.Owner),
			CreatedAt:     createdAt,
			AnswerPreview: reduceBody(answer.Body),
		})
	}
	return newAnswers, nil
}

func reduceBody(full string) string {
	if len(full) <= 200 {
		return full
	}
	return full[:200]
}

func checkAndReturnUsername(user *dto.ShallowUser) string {
	if user == nil {
		return "unknown"
	}
	return user.Username
}
