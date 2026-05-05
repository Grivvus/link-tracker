package clients

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestStackoverflowClient_FetchQuestion(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{
				"last_activity_date": 1710075600,
				"link": "https://stackoverflow.com/questions/123",
				"title": "Example question"
			}`), nil
		}),
	}

	got, err := client.FetchQuestion(
		context.TODO(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "https://stackoverflow.com/questions/123", got.URL)
	require.Equal(t, "Example question", got.Title)
	require.Equal(t, int64(1710075600), got.LastActivityDate)
}

func TestStackoverflowClient_FetchQuestionComments_HappyPath(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{
				"items": [{
					"comment_id": 11,
					"creation_date": 1710075601,
					"body": "comment body",
					"owner": {
						"display_name": "alice",
						"user_id": 42
					}
				}]
			}`), nil
		}),
	}

	got, err := client.FetchQuestionComments(context.TODO(), 123)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(11), got[0].CommentID)
	require.Equal(t, "comment body", got[0].Body)
	require.Equal(t, int64(1710075601), got[0].CreationDate)
	require.NotNil(t, got[0].Owner)
	require.Equal(t, "alice", got[0].Owner.Username)
	require.Equal(t, int64(42), got[0].Owner.UserID)
}

func TestStackoverflowClient_FetchQuestionAnswers(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{
				"items": [{
					"answer_id": 77,
					"creation_date": 1710075602,
					"last_activity_date": 1710075700,
					"body": "answer body",
					"owner": {
						"display_name": "bob",
						"user_id": 84
					}
				}]
			}`), nil
		}),
	}

	got, err := client.FetchQuestionAnswers(context.TODO(), 123)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(77), got[0].AnswerID)
	require.Equal(t, "answer body", got[0].Body)
	require.Equal(t, int64(1710075602), got[0].CreationDate)
	require.Equal(t, int64(1710075700), got[0].LastActivityDate)
	require.NotNil(t, got[0].Owner)
	require.Equal(t, "bob", got[0].Owner.Username)
	require.Equal(t, int64(84), got[0].Owner.UserID)
}

func TestStackoverflowClient_FetchQuestion_Non2xxResponse(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusInternalServerError, `{"error":"internal"}`), nil
		}),
	}

	_, err := client.FetchQuestion(
		context.TODO(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestStackoverflowClient_FetchQuestion_InvalidJSON(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{"last_activity_date":"oops"`), nil
		}),
	}

	_, err := client.FetchQuestion(
		context.TODO(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestStackoverflowClient_FetchQuestion_Non2XXResponse(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 100), func(i int) bool {
		return false
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusInternalServerError, `{"message":"server is down}"`), nil
		}),
	}

	_, err := client.FetchQuestion(
		context.TODO(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestStackoverflowClient_FetchQuestion_Timeout(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(1, 1), func(int) bool {
		return false
	})
	client.httpClient.Timeout = 20 * time.Millisecond
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	start := time.Now()
	_, err := client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)

	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestStackoverflowClient_FetchQuestion_TimeoutRetriesAndEventuallySucceeds(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testRCFG(2, 1), func(int) bool {
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
			"last_activity_date": 1710075600,
			"link": "https://stackoverflow.com/questions/123",
			"title": "Example question"
		}`), nil
	})

	got, err := client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{
			QuestionID: 123,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, "https://stackoverflow.com/questions/123", got.URL)
	require.Equal(t, "Example question", got.Title)
}

func TestStackoverflowClient_FetchQuestion_CircuitBreakerOpensAndRejectsRequests(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testBreakerRCFG(), func(statusCode int) bool {
		return statusCode >= http.StatusInternalServerError
	})
	requests := 0
	client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		return newHTTPResponse(http.StatusInternalServerError, `{"message":"server error"}`), nil
	})

	_, err := client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{QuestionID: 123},
	)
	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, gobreaker.StateOpen, client.breaker.State())

	_, err = client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{QuestionID: 123},
	)
	require.ErrorIs(t, err, gobreaker.ErrOpenState)
	require.Equal(t, 1, requests)
}

func TestStackoverflowClient_FetchQuestion_CircuitBreakerClassifiesHTTPStatusCodes(t *testing.T) {
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
			client := NewStackoverflowClient(testClientLogger(), testBreakerRCFG(), func(statusCode int) bool {
				return statusCode >= http.StatusInternalServerError
			})
			requests := 0
			client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return newHTTPResponse(tc.statusCode, `{"message":"request failed"}`), nil
				}
				return newHTTPResponse(http.StatusOK, `{
					"last_activity_date": 1710075600,
					"link": "https://stackoverflow.com/questions/123",
					"title": "Example question"
				}`), nil
			})

			_, err := client.FetchQuestion(
				context.Background(),
				domain.StackoverflowURLInfo{QuestionID: 123},
			)
			require.ErrorIs(t, err, ports.ErrBadResponse)

			if tc.wantOpen {
				require.Equal(t, gobreaker.StateOpen, client.breaker.State())

				_, err = client.FetchQuestion(
					context.Background(),
					domain.StackoverflowURLInfo{QuestionID: 123},
				)
				require.ErrorIs(t, err, gobreaker.ErrOpenState)
				require.Equal(t, 1, requests)
				return
			}

			require.Equal(t, gobreaker.StateClosed, client.breaker.State())

			got, err := client.FetchQuestion(
				context.Background(),
				domain.StackoverflowURLInfo{QuestionID: 123},
			)
			require.NoError(t, err)
			require.Equal(t, "Example question", got.Title)
			require.Equal(t, 2, requests)
			require.Equal(t, gobreaker.StateClosed, client.breaker.State())
		})
	}
}

func TestStackoverflowClient_FetchQuestion_CircuitBreakerAllowsHalfOpenProbeAndClosesOnSuccess(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger(), testBreakerRCFG(), func(statusCode int) bool {
		return statusCode >= http.StatusInternalServerError
	})
	requests := 0
	client.httpClient.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return newHTTPResponse(http.StatusInternalServerError, `{"message":"server error"}`), nil
		}
		return newHTTPResponse(http.StatusOK, `{
			"last_activity_date": 1710075600,
			"link": "https://stackoverflow.com/questions/123",
			"title": "Example question"
		}`), nil
	})

	_, err := client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{QuestionID: 123},
	)
	require.ErrorIs(t, err, ports.ErrBadResponse)
	require.Equal(t, gobreaker.StateOpen, client.breaker.State())

	time.Sleep(1100 * time.Millisecond)

	got, err := client.FetchQuestion(
		context.Background(),
		domain.StackoverflowURLInfo{QuestionID: 123},
	)
	require.NoError(t, err)
	require.Equal(t, "Example question", got.Title)
	require.Equal(t, 2, requests)
	require.Equal(t, gobreaker.StateClosed, client.breaker.State())
}
