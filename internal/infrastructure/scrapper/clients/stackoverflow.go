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
const stackoverflowAPIVersion = 2.3
const stackoverflowSite = "stackoverflow"

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildStackoverflowURL(info), nil)
	if err != nil {
		c.logger.Error("error creating request", "url info", info)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	c.logger.Info("request was built", "url", req.URL.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"error executing request", "url info", info, "err", err,
		)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: exact error %w", ports.ErrBadResponse, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	c.logger.Info("got response", "code", resp.StatusCode)
	if resp.StatusCode >= http.StatusBadRequest {
		c.logger.Error(
			"getting error-response",
			"url info", info, "status_code", resp.StatusCode,
		)
		return dto.StackoverflowQuestion{}, ports.ErrBadResponse
	}

	respDTO := dto.StackoverflowQuestion{}
	err = json.NewDecoder(resp.Body).Decode(&respDTO)
	if err != nil {
		c.logger.Error(
			"error decoding response",
			"url info", info, "err", err,
		)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: can't decode response", ports.ErrBadResponse,
		)
	}
	return respDTO, nil
}

func buildStackoverflowURL(info domain.StackoverflowURLInfo) string {
	return fmt.Sprintf(
		"%s/%f/%s/%d?%s",
		stackoverflowAPIURL, stackoverflowAPIVersion,
		"questions", info.QuestionID, stackoverflowSite,
	)
}
