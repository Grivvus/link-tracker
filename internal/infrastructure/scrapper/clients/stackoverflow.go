package clients

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker/v2"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/domain"
	"link-tracker/internal/infrastructure"
	"link-tracker/internal/infrastructure/dto"
)

const stackoverflowAPIURL = "https://api.stackexchange.com"
const stackoverflowAPIVersion = "2.3"
const stackoverflowSite = "stackoverflow"
const stackoverflowBodyFilter = "withbody"

type StackoverflowClient struct {
	logger        *slog.Logger
	httpClient    *http.Client
	retrier       *retry.Retrier
	breaker       *gobreaker.TwoStepCircuitBreaker[struct{}]
	shouldRetryFn func(statusCode int) bool
}

func NewStackoverflowClient(
	logger *slog.Logger,
	rcfg infrastructure.ReliabiltyConfig,
	shouldRetryFn func(int) bool,
) *StackoverflowClient {
	retrier := retry.New(
		retry.Attempts(uint(rcfg.RetryPolicy)),
		retry.Delay(time.Duration(rcfg.RetryPolicyDelayMS)*time.Millisecond),
	)
	return &StackoverflowClient{
		logger: logger,
		httpClient: &http.Client{
			Timeout: time.Duration(rcfg.ExternalServicesTimeoutSecond) * time.Second,
		},
		retrier: retrier,
		breaker: infrastructure.NewBreaker(
			logger, rcfg, "stackoverflow-api",
			func(err error) bool {
				httpErr, ok := errors.AsType[HTTPError](err)
				return !ok || httpErr.Code < 500
			},
		),
		shouldRetryFn: shouldRetryFn,
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
	return infrastructure.ExecuteWithBreaker(c.breaker, func() (dto.StackoverflowQuestion, error) {
		return doRequestAndMapResult[dto.StackoverflowQuestion](
			c.logger, c.httpClient, req, c.retrier, c.shouldRetryFn,
		)
	})
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
	return infrastructure.ExecuteWithBreaker(c.breaker, func() ([]dto.StackoverflowAnswer, error) {
		answers, err := doRequestAndMapResult[dto.StackoverflowAnswers](
			c.logger, c.httpClient, req, c.retrier, c.shouldRetryFn,
		)
		if err != nil {
			return nil, err
		}
		return answers.Items, nil
	})
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
	return infrastructure.ExecuteWithBreaker(c.breaker, func() ([]dto.StackoverflowComment, error) {
		comments, err := doRequestAndMapResult[dto.StackoverflowComments](
			c.logger, c.httpClient, req, c.retrier, c.shouldRetryFn,
		)
		if err != nil {
			return nil, err
		}
		return comments.Items, nil
	})
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
