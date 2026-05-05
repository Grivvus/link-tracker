package clients

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func testClientLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testRCFG(n int, delayMS int64) infrastructure.ReliabiltyConfig {
	return infrastructure.ReliabiltyConfig{
		RetryPolicy:                   n,
		RetryPolicyDelayMS:            delayMS,
		BreakerHalfopenMaxRequests:    1,
		BreakerTimeoutSecond:          1,
		BreakerBucketPeriodSecond:     1,
		BreakerIntervalSecond:         1,
		BreakerFailureTreshold:        100,
		ExternalServicesTimeoutSecond: 1,
		RateLimiterRequestsPerSecond:  1,
		ReateLimiterMaxBurst:          1,
	}
}

func testBreakerRCFG() infrastructure.ReliabiltyConfig {
	return infrastructure.ReliabiltyConfig{
		RetryPolicy:                   1,
		RetryPolicyDelayMS:            1,
		ExternalServicesTimeoutSecond: 1,
		BreakerHalfopenMaxRequests:    1,
		BreakerTimeoutSecond:          1,
		BreakerBucketPeriodSecond:     1,
		BreakerIntervalSecond:         1,
		BreakerFailureTreshold:        50,
		RateLimiterRequestsPerSecond:  1,
		ReateLimiterMaxBurst:          1,
	}
}

func TestGitHubClient_FetchRepo_Non2xxResponse(t *testing.T) {
	client := NewGitHubClient(testClientLogger(), testRCFG(1, 100), func(sc int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusBadGateway, `{"message":"bad gateway"}`), nil
		}),
	}

	_, err := client.FetchRepo(context.TODO(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestGitHubClient_FetchRepo_InvalidJSON(t *testing.T) {
	client := NewGitHubClient(testClientLogger(), testRCFG(1, 100), func(sc int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{"name":"repo",`), nil
		}),
	}

	_, err := client.FetchRepo(context.TODO(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestGitHubClient_FetchRepo_Timeout(t *testing.T) {
	client := NewGitHubClient(testClientLogger(), testRCFG(1, 1), func(int) bool {
		return false
	})
	client.httpClient.Timeout = 20 * time.Millisecond
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	start := time.Now()
	_, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})

	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestGitHubClient_FetchRepo_TimeoutRetriesAndEventuallySucceeds(t *testing.T) {
	client := NewGitHubClient(testClientLogger(), testRCFG(2, 1), func(int) bool {
		return false
	})
	client.httpClient.Timeout = 20 * time.Millisecond

	attempts := 0
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		return newHTTPResponse(http.StatusOK, `{
			"name":"repo",
			"updated_at":"2026-05-04T00:00:00Z",
			"html_url":"https://github.com/user/repo"
		}`), nil
	})

	got, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, "repo", got.Name)
	require.Equal(t, "https://github.com/user/repo", got.URL)
}

func TestGitHubClient_FetchRepo_CircuitBreakerOpensAndRejectsRequests(t *testing.T) {
	client := NewGitHubClient(testClientLogger(), testBreakerRCFG(), func(statusCode int) bool {
		return statusCode >= http.StatusInternalServerError
	})
	requests := 0
	client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		return newHTTPResponse(http.StatusInternalServerError, `{"message":"server error"}`), nil
	})

	_, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, gobreaker.StateOpen, client.breaker.State())

	_, err = client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.ErrorIs(t, err, gobreaker.ErrOpenState)
	require.Equal(t, 1, requests)
}

func TestGitHubClient_FetchRepo_CircuitBreakerClassifiesHTTPStatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		wantOpen   bool
	}{
		{
			name:       "bad request does not open breaker",
			statusCode: http.StatusBadRequest,
			wantOpen:   false,
		},
		{
			name:       "not found does not open breaker",
			statusCode: http.StatusNotFound,
			wantOpen:   false,
		},
		{
			name:       "too many requests does not open breaker",
			statusCode: http.StatusTooManyRequests,
			wantOpen:   false,
		},
		{
			name:       "internal server error opens breaker",
			statusCode: http.StatusInternalServerError,
			wantOpen:   true,
		},
		{
			name:       "bad gateway opens breaker",
			statusCode: http.StatusBadGateway,
			wantOpen:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewGitHubClient(testClientLogger(), testBreakerRCFG(), func(statusCode int) bool {
				return statusCode >= http.StatusInternalServerError
			})
			requests := 0
			client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return newHTTPResponse(tc.statusCode, `{"message":"request failed"}`), nil
				}
				return newHTTPResponse(http.StatusOK, `{
					"name":"repo",
					"updated_at":"2026-05-04T00:00:00Z",
					"html_url":"https://github.com/user/repo"
				}`), nil
			})

			_, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
				Username: "user",
				RepoName: "repo",
			})
			require.ErrorIs(t, err, ports.ErrBadResponse)

			if tc.wantOpen {
				require.Equal(t, gobreaker.StateOpen, client.breaker.State())

				_, err = client.FetchRepo(context.Background(), domain.GitHubURLInfo{
					Username: "user",
					RepoName: "repo",
				})
				require.ErrorIs(t, err, gobreaker.ErrOpenState)
				require.Equal(t, 1, requests)
				return
			}

			require.Equal(t, gobreaker.StateClosed, client.breaker.State())

			got, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
				Username: "user",
				RepoName: "repo",
			})
			require.NoError(t, err)
			require.Equal(t, "repo", got.Name)
			require.Equal(t, 2, requests)
			require.Equal(t, gobreaker.StateClosed, client.breaker.State())
		})
	}
}

func TestGitHubClient_FetchRepo_CircuitBreakerAllowsHalfOpenProbeAndClosesOnSuccess(t *testing.T) {
	rcfg := testBreakerRCFG()
	rcfg.BreakerTimeoutSecond = 1
	client := NewGitHubClient(testClientLogger(), rcfg, func(statusCode int) bool {
		return statusCode >= http.StatusInternalServerError
	})
	requests := 0
	client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return newHTTPResponse(http.StatusInternalServerError, `{"message":"server error"}`), nil
		}
		return newHTTPResponse(http.StatusOK, `{
			"name":"repo",
			"updated_at":"2026-05-04T00:00:00Z",
			"html_url":"https://github.com/user/repo"
		}`), nil
	})

	_, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, gobreaker.StateOpen, client.breaker.State())

	time.Sleep(1100 * time.Millisecond)

	got, err := client.FetchRepo(context.Background(), domain.GitHubURLInfo{
		Username: "user",
		RepoName: "repo",
	})
	require.NoError(t, err)
	require.Equal(t, "repo", got.Name)
	require.Equal(t, 2, requests)
	require.Equal(t, gobreaker.StateClosed, client.breaker.State())
}
