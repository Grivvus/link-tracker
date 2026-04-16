package clients

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
)

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
