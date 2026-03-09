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
	url *url.URL,
) (dto.StackoverflowQuestion, error) {
	if !strings.Contains(url.Path, "questions") {
		c.logger.Error("url didn't contains 'questions'", "url", url)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: url-path must starts with questions/", ports.ErrBadParams,
		)
	}
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		c.logger.Error("error creating request", "url", url)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: %w", ports.ErrBadParams, err,
		)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"error executing request", "url", url, "err", err,
		)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: exact error %w", ports.ErrBadResponse, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		c.logger.Error(
			"getting error-response",
			"url", url, "status_code", resp.StatusCode,
		)
		return dto.StackoverflowQuestion{}, ports.ErrBadResponse
	}

	respDTO := dto.StackoverflowQuestion{}
	err = json.NewDecoder(resp.Body).Decode(&respDTO)
	if err != nil {
		c.logger.Error(
			"error decoding response",
			"url", url, "err", err,
		)
		return dto.StackoverflowQuestion{}, fmt.Errorf(
			"%w: can't decode response", ports.ErrBadResponse,
		)
	}
	return respDTO, nil
}
