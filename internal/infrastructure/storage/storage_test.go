package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"link-tracker/internal/application/scrapper/repository"
	"link-tracker/internal/domain"
)

func mustRegisterChat(t *testing.T, repo repository.ChatRepository, chatID int64) {
	t.Helper()
	require.NoError(t, repo.Register(context.Background(), chatID))
}

func mustNewLink(t *testing.T, rawURL string) domain.Link {
	t.Helper()
	link, err := domain.NewLink(rawURL)
	require.NoError(t, err)
	return link
}

func TestStorage_Subscribe_DuplicateSubscription(t *testing.T) {
	ctx := context.Background()
	st := New()
	chatRepo := st
	subRepo := st
	chatID := int64(1)
	link := mustNewLink(t, "https://github.com/u/repo-1")

	mustRegisterChat(t, chatRepo, chatID)

	_, err := subRepo.Subscribe(ctx, link, chatID, []string{"go"})
	require.NoError(t, err)
	_, err = subRepo.Subscribe(ctx, link, chatID, []string{"go"})
	require.ErrorIs(t, err, repository.ErrEntityAlreadyExists)
}

func TestStorage_AllLinkSubscriptions_ReturnsOnlyFollowers(t *testing.T) {
	ctx := context.Background()
	st := New()
	chatRepo := st
	subRepo := st

	chatID1 := int64(100)
	chatID2 := int64(200)
	mustRegisterChat(t, chatRepo, chatID1)
	mustRegisterChat(t, chatRepo, chatID2)

	link1 := mustNewLink(t, "https://github.com/user/repo-followed")
	link2 := mustNewLink(t, "https://github.com/user/repo-other")

	_, err := subRepo.Subscribe(ctx, link1, chatID1, []string{"go"})
	require.NoError(t, err)
	_, err = subRepo.Subscribe(ctx, link2, chatID2, []string{"java"})
	require.NoError(t, err)

	subs, err := subRepo.AllLinkSubscriptions(ctx, link1)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, chatID1, subs[0].ChatID)
}

func TestStorage_AllChatSubscriptions_EmptyWhenNoLinks(t *testing.T) {
	ctx := context.Background()
	st := New()
	chatRepo := st
	subRepo := st
	chatID := int64(777)
	mustRegisterChat(t, chatRepo, chatID)

	subs, err := subRepo.AllChatSubscriptions(ctx, chatID)
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestStorage_GetAllLinks_AfterSubscriptions(t *testing.T) {
	ctx := context.Background()
	st := New()
	chatRepo := st
	subRepo := st
	chatID := int64(42)
	mustRegisterChat(t, chatRepo, chatID)

	urls := []string{
		fmt.Sprintf("https://github.com/u/repo-%d", 10),
		fmt.Sprintf("https://github.com/u/repo-%d", 20),
	}
	for _, rawURL := range urls {
		link := mustNewLink(t, rawURL)
		_, err := subRepo.Subscribe(ctx, link, chatID, []string{"x"})
		require.NoError(t, err)
	}

	linksChan, errChan := subRepo.GetAllLinks(ctx)
	errs := make([]error, 0)
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs)
	links := make([]domain.Link, 0)
	for link := range linksChan {
		links = append(links, link)
	}
	assert.Len(t, links, 2)
}
