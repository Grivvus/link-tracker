package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestLink_Github_ValidURL(t *testing.T) {
	t.Parallel()

	urls := []string{
		"http://github.com/golang/go/",
		"https://github.com/golang/go/",
		"github.com/golang/go/",
	}

	for _, url := range urls {
		l, err := domain.NewLink(url)
		require.NoError(t, err)
		isGH, ghInfo := l.GitHubURLInfo()
		assert.True(t, isGH)
		require.NotNil(t, ghInfo)
		assert.Equal(t, "golang", ghInfo.Username)
		assert.Equal(t, "go", ghInfo.RepoName)
	}
}

func TestLink_Github_InvalidURL(t *testing.T) {
	t.Parallel()

	urls := []string{
		"ftp://github.com/golang/go/",
		"smtp://github.com/golang/go/",
		"github.com/golang/go/issues/123",
	}

	for _, url := range urls {
		_, err := domain.NewLink(url)
		assert.ErrorIs(t, err, domain.ErrBadURL)
	}
}

func TestLink_Stackoverflow_ValidURL(t *testing.T) {
	t.Parallel()
	urls := []string{
		"stackoverflow.com/questions/123",
		"https://stackoverflow.com/questions/123",
		"http://stackoverflow.com/questions/123/",
	}
	for _, url := range urls {
		l, err := domain.NewLink(url)
		require.NoError(t, err)
		isSt, stackoverflowInfo := l.StackoverflowURLInfo()
		assert.True(t, isSt)
		require.NotNil(t, stackoverflowInfo)
		assert.Equal(t, int64(123), stackoverflowInfo.QuestionID)
	}
}

func TestLink_Stackoverflow_InvalidURL(t *testing.T) {
	t.Parallel()
	urls := []string{
		"stackoverflow.ru/questions/123",
		"stackoverflow.ru/questions/123/321",
		"stackoverflow.ru/questions/",
		"smtp://stackoverflow.com/questions/123",
		"grpc://stackoverflow.com/questions/123/",
	}
	for _, url := range urls {
		_, err := domain.NewLink(url)
		assert.ErrorIs(t, err, domain.ErrBadURL)
	}
}
