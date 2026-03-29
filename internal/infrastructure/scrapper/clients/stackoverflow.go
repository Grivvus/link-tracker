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

const stackoverflowAPIURL = "https://api.stackexchange.com"
const stackoverflowAPIVersion = "2.3"
const stackoverflowSite = "stackoverflow"
const stackoverflowBodyFilter = "withbody"

type StackoverflowClient struct {
	logger     *slog.Logger
	httpClient *http.Client
}

func NewStackoverflowClient(logger *slog.Logger) *StackoverflowClient {
	return &StackoverflowClient{
		logger:     logger,
		httpClient: &http.Client{},
	}
}

func (c *StackoverflowClient) FetchQuestion(
	ctx context.Context, info domain.StackoverflowURLInfo,
) (dto.StackoverflowQuestion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildStackoverflowQuestionURL(info), nil)
	if err != nil {
		c.logger.Error("error creating request", "url info", info)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	c.logger.Info("request was built", "url", req.URL.String())
	question, err := doRequestAndMapResult[dto.StackoverflowQuestion](c.logger, c.httpClient, req)
	if err != nil {
		return dto.StackoverflowQuestion{}, err
	}
	return question, nil
}

func (c *StackoverflowClient) FetchQuestionAnswers(
	ctx context.Context, questionID int64,
) ([]dto.StackoverflowAnswer, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		buildStackoverflowQuestionContentURL(questionID, "answers"),
		nil,
	)
	if err != nil {
		c.logger.Error("error creating request", "question id", questionID)
		return nil, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	c.logger.Info("request was built", "url", req.URL.String())
	answers, err := doRequestAndMapResult[dto.StackoverflowAnswers](c.logger, c.httpClient, req)
	if err != nil {
		return nil, err
	}
	return answers.Items, nil
}

func (c *StackoverflowClient) FetchQuestionComments(
	ctx context.Context, questionID int64,
) ([]dto.StackoverflowComment, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		buildStackoverflowQuestionContentURL(questionID, "comments"),
		nil,
	)
	if err != nil {
		c.logger.Error("error creating request", "err", err)
		return nil, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	c.logger.Info("request was built", "url", req.URL.String())
	comments, err := doRequestAndMapResult[dto.StackoverflowComments](c.logger, c.httpClient, req)
	if err != nil {
		return nil, err
	}
	return comments.Items, nil
}

type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func doRequestAndMapResult[T any](logger *slog.Logger, d doer, r *http.Request) (T, error) {
	var zero T
	resp, err := d.Do(r)
	if err != nil {
		logger.Error(
			"error executing request", "err", err,
		)
		return zero, fmt.Errorf(
			"%w: exact error %w", ports.ErrBadResponse, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	logger.Info("got response", "code", resp.StatusCode)
	if resp.StatusCode >= http.StatusBadRequest {
		logger.Error(
			"getting error-response",
			"status_code", resp.StatusCode,
		)
		return zero, ports.ErrBadResponse
	}

	err = json.NewDecoder(resp.Body).Decode(&zero)
	if err != nil {
		logger.Error(
			"error decoding response",
			"err", err,
		)
		return zero, fmt.Errorf(
			"%w: can't decode response", ports.ErrBadResponse,
		)
	}
	return zero, nil
}

func buildStackoverflowQuestionURL(info domain.StackoverflowURLInfo) string {
	return fmt.Sprintf(
		"%s/%s/questions/%d?site=%s",
		stackoverflowAPIURL, stackoverflowAPIVersion,
		info.QuestionID, stackoverflowSite,
	)
}

func buildStackoverflowQuestionContentURL(questionID int64, contentType string) string {
	return fmt.Sprintf(
		"%s/%s/questions/%d/%s?site=%s&filter=%s&order=%v&sort=%v",
		stackoverflowAPIURL, stackoverflowAPIVersion,
		questionID, contentType,
		stackoverflowSite, stackoverflowBodyFilter,
		"desc", "creation",
	)
}
