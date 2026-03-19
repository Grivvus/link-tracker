package adapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
)

var testID atomic.Int64

func nextID() int64 {
	return testID.Add(1) + 1000
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestScrapperAdapter_AddLinkAndListTrackedLinks_SavesData(t *testing.T) {
	ctx := context.Background()
	chatID := nextID()
	rawURL := fmt.Sprintf("https://github.com/user/repo-%d", chatID)
	st := storage.New()
	adapter := NewScrapperAdapter(
		testLogger(),
		st, st,
	)

	require.NoError(t, adapter.RegisterTGChat(ctx, chatID))

	added, err := adapter.AddLink(ctx, model.TrackLinkCmd{
		ChatID: chatID,
		RawURL: rawURL,
		Tags:   []string{"go", "backend"},
	})
	require.NoError(t, err)
	assert.Equal(t, rawURL, added.RawURL)
	assert.Positive(t, added.LinkID)

	links, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{
		ChatID: chatID,
	})
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, rawURL, links[0].RawURL)
	assert.Len(t, links[0].Tags, 2)
}

func TestScrapperAdapter_AddLink_InvalidURL_ReturnsBadParams(t *testing.T) {
	ctx := context.Background()
	chatID := nextID()
	st := storage.New()
	adapter := NewScrapperAdapter(
		testLogger(),
		st, st,
	)

	require.NoError(t, adapter.RegisterTGChat(ctx, chatID))

	_, err := adapter.AddLink(ctx, model.TrackLinkCmd{
		ChatID: chatID,
		RawURL: "abcd://github.com/user/repo",
		Tags:   []string{"go"},
	})
	require.ErrorIs(t, err, ports.ErrBadParams)
}

func TestScrapperAdapter_AddLink_DuplicateSubscription_ReturnsAlreadyTracked(t *testing.T) {
	ctx := context.Background()
	chatID := nextID()
	rawURL := fmt.Sprintf("https://github.com/user/repo-dup-%d", chatID)
	st := storage.New()
	adapter := NewScrapperAdapter(
		testLogger(),
		st, st,
	)

	require.NoError(t, adapter.RegisterTGChat(ctx, chatID))

	cmd := model.TrackLinkCmd{
		ChatID: chatID,
		RawURL: rawURL,
		Tags:   []string{"go"},
	}
	_, err := adapter.AddLink(ctx, cmd)
	require.NoError(t, err)

	_, err = adapter.AddLink(ctx, cmd)
	require.ErrorIs(t, err, ports.ErrLinkAlreadyTracked)
}

func TestScrapperAdapter_GetTrackedLinks_NoSubscriptions_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	chatID := nextID()
	st := storage.New()
	adapter := NewScrapperAdapter(
		testLogger(),
		st, st,
	)

	require.NoError(t, adapter.RegisterTGChat(ctx, chatID))

	links, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{
		ChatID: chatID,
	})
	require.NoError(t, err)
	assert.Empty(t, links)
}
