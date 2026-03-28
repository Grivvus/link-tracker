package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
		return a.fetchGH(ctx, link)
	case "stackoverflow.com":
		return a.fetchStackoverflow(ctx, link)
	default:
		return model.LinkState{}, fmt.Errorf("%w: url: %s", ports.ErrBadURL, link.URL.String())
	}
}

func (a *LinkStateAdapter) fetchGH(ctx context.Context, link domain.Link) (model.LinkState, error) {
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
		UpdateInfo: "abcd",
		UpdatedAt:  ghDTO.UpdatedAt,
		Title:      ghDTO.Name,
	}, nil
}

func (a *LinkStateAdapter) fetchStackoverflow(
	ctx context.Context, link domain.Link,
) (model.LinkState, error) {
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
	if !stoverflowDTO.LastActivityDate.After(link.LastUpdate) {
		// no update since last check
		// zero value for UpdatedAt is fine
		// it will be checked at the next step in LinkPollingUseCase
		return model.LinkState{}, nil
	}
	newComments, err := a.newCommentsSince(ctx, stinfo.QuestionID, link.LastUpdate)
	_ = newComments
	if err != nil {
		return model.LinkState{}, fmt.Errorf(
			"%w: can't fetch new comments. Initail err: %w",
			ports.ErrBadResponse, err,
		)
	}
	newAnswers, err := a.newAnswersSince(ctx, stinfo.QuestionID, link.LastUpdate)
	_ = newAnswers
	if err != nil {
		return model.LinkState{}, fmt.Errorf(
			"%w: can't fetch new answers. Initail err: %w",
			ports.ErrBadResponse, err,
		)
	}
	return model.LinkState{
		UpdateInfo: "abcd",
		UpdatedAt:  stoverflowDTO.LastActivityDate,
		Title:      stoverflowDTO.Title,
	}, nil
}

func (a *LinkStateAdapter) newCommentsSince(
	ctx context.Context, questionID int64, since time.Time,
) ([]model.StackoverflowNewComment, error) {
	return []model.StackoverflowNewComment{}, nil
}

func (a *LinkStateAdapter) newAnswersSince(
	ctx context.Context, questionID int64, since time.Time,
) ([]model.StackoverflowNewAnswer, error) {
	return []model.StackoverflowNewAnswer{}, nil
}
