package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/avast/retry-go/v5"
	"link-tracker/internal/application/ports"
)

type HTTPError struct {
	Code int
	Err  error
}

func (err HTTPError) Error() string {
	return err.Err.Error()
}

type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func doRequestAndMapResult[T any](
	logger *slog.Logger, d doer,
	r *http.Request,
	retrier *retry.Retrier, shouldRetry func(statusCode int) bool,
) (T, error) {
	var zero T
	var respData []byte
	err := retrier.Do(func() error {
		r, err := d.Do(r)
		if err != nil {
			return err
		}
		defer func() { _ = r.Body.Close() }()
		if r.StatusCode >= 400 {
			if !shouldRetry(r.StatusCode) {
				return retry.Unrecoverable(HTTPError{
					Code: r.StatusCode,
					Err:  fmt.Errorf("unrecoverable status code: %v", r.StatusCode),
				})
			}
			return HTTPError{
				Code: r.StatusCode,
				Err:  fmt.Errorf("bad response, should retry; status code: %v", r.StatusCode),
			}
		}
		respData, err = io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error(
			"error executing request", "err", err,
		)
		return zero, fmt.Errorf(
			"%w: exact error %w", ports.ErrBadResponse, err,
		)
	}
	err = json.NewDecoder(bytes.NewReader(respData)).Decode(&zero)
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
