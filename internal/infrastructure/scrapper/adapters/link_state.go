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
			"%w: exected valid GitHubURLInfo", ports.ErrBadParams,
		)
	}
	ghDTO, err := a.ghClient.FetchRepo(ctx, *ghinfo)
	if err != nil {
		return nil, err
	}
	return []model.LinkState{{
		UpdateInfo: "abcd",
		UpdatedAt:  ghDTO.UpdatedAt,
		Title:      ghDTO.Name,
	}}, nil
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
	lastActivityDate := time.Unix(stoverflowDTO.LastActivityDate, 0)
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
		})
	}
	for _, answer := range newAnswers {
		updates = append(updates, model.LinkState{
			UpdateInfo: answer.AnswerPreview,
			UpdatedAt:  answer.CreatedAt,
			Title:      answer.QuestionName,
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
		createdAt := time.Unix(comment.CreationDate, 0)
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
		createdAt := time.Unix(answer.CreationDate, 0)
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
