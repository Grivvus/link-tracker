package clients

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/stretchr/testify/require"
	"link-tracker/internal/application/ports"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestRetrier(attempts uint) *retry.Retrier {
	return retry.New(
		retry.Attempts(attempts),
		retry.Delay(time.Millisecond),
	)
}

func TestDoRequestAndMapResult_RetriesRetryableStatusAndEventuallySucceeds(t *testing.T) {
	attempts := 0
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	type responsePayload struct {
		Value string `json:"value"`
	}

	got, err := doRequestAndMapResult[responsePayload](
		testClientLogger(),
		doerFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return newHTTPResponse(http.StatusBadGateway, `{"message":"bad gateway"}`), nil
			}
			return newHTTPResponse(http.StatusOK, `{"value":"ok"}`), nil
		}),
		req,
		newTestRetrier(3),
		func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		},
	)

	require.NoError(t, err)
	require.Equal(t, responsePayload{Value: "ok"}, got)
	require.Equal(t, 3, attempts)
}

func TestDoRequestAndMapResult_DoesNotRetryOnUnrecoverableStatus(t *testing.T) {
	attempts := 0
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	_, err = doRequestAndMapResult[map[string]string](
		testClientLogger(),
		doerFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			return newHTTPResponse(http.StatusNotFound, `{"message":"not found"}`), nil
		}),
		req,
		newTestRetrier(3),
		func(int) bool {
			return false
		},
	)

	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, 1, attempts)
}

func TestDoRequestAndMapResult_RetryableStatusFollowedByUnrecoverableStatusReturnsError(t *testing.T) {
	attempts := 0
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	_, err = doRequestAndMapResult[map[string]string](
		testClientLogger(),
		doerFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return newHTTPResponse(http.StatusInternalServerError, `{"message":"temporary failure"}`), nil
			}
			return newHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}),
		req,
		newTestRetrier(3),
		func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		},
	)

	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, 2, attempts)
}
