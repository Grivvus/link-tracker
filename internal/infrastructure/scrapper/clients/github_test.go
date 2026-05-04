package clients

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

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
		RetryPolicy:        n,
		RetryPolicyDelayMS: delayMS,
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
