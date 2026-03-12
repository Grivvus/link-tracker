package clients

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestStackoverflowClient_FetchQuestion_Non2xxResponse(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger())
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusInternalServerError, `{"error":"internal"}`), nil
		}),
	}

	_, err := client.FetchQuestion(domain.StackoverflowURLInfo{
		QuestionID: 123,
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
}

func TestStackoverflowClient_FetchQuestion_InvalidJSON(t *testing.T) {
	client := NewStackoverflowClient(testClientLogger())
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusOK, `{"last_activity_date":"oops"`), nil
		}),
	}

	_, err := client.FetchQuestion(domain.StackoverflowURLInfo{
		QuestionID: 123,
	})
	require.ErrorIs(t, err, ports.ErrBadResponse)
}
