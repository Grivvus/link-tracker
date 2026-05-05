package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
)

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
				return retry.Unrecoverable(fmt.Errorf("unrecoverable status code: %v", r.StatusCode))
			}
			return fmt.Errorf("bad response, should retry; status code: %v", r.StatusCode)
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

func executeWithBreaker[T any](
	cb *gobreaker.TwoStepCircuitBreaker[struct{}],
	op func() (T, error),
) (T, error) {
	done, err := cb.Allow()
	if err != nil {
		var zero T
		return zero, err
	}

	result, err := op()
	done(err)

	return result, err
}

func newBreaker(logger *slog.Logger, rcfg infrastructure.ReliabiltyConfig, breakerName string) *gobreaker.TwoStepCircuitBreaker[struct{}] {
	breaker := gobreaker.NewTwoStepCircuitBreaker[struct{}](gobreaker.Settings{
		Name:         breakerName,
		MaxRequests:  rcfg.BreakerHalfopenMaxRequests,
		Interval:     time.Duration(rcfg.BreakerIntervalSecond) * time.Second,
		Timeout:      time.Duration(rcfg.BreakerTimeoutSecond) * time.Second,
		BucketPeriod: time.Duration(rcfg.BreakerBucketPeriodSecond) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests == 0 {
				return false
			}
			failurePercentage := float64(counts.TotalFailures) / float64(counts.Requests) * 100
			return failurePercentage >= float64(rcfg.BreakerFailureTreshold)
		},
		IsSuccessful: func(err error) bool {
			return err == nil
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Warn("circuit breaker state changed", "name", name, "from", from, "to", to)
		},
	})
	return breaker
}
