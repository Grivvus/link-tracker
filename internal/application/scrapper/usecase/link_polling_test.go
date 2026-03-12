package usecase

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type fakeBotPort struct {
	events []model.LinkUpdatedEvent
}

func (f *fakeBotPort) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	f.events = append(f.events, event)
	return nil
}

type fakeSubscriptionRepo struct {
	updatedLinks []domain.Link
}

func (f *fakeSubscriptionRepo) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}

func (f *fakeSubscriptionRepo) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}

func (f *fakeSubscriptionRepo) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *fakeSubscriptionRepo) GetAllLinks(
	ctx context.Context,
) ([]domain.Link, error) {
	return nil, nil
}

func (f *fakeSubscriptionRepo) UpdateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
	f.updatedLinks = append(f.updatedLinks, link)
	return link, nil
}

func (f *fakeSubscriptionRepo) FindLinkByURL(
	ctx context.Context, parsedURL *url.URL,
) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *fakeSubscriptionRepo) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
	return nil, nil
}

func (f *fakeSubscriptionRepo) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	return nil, nil
}

func testUsecaseLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLinkPollingUseCase_UpdateAndNotify_NewerUpdate(t *testing.T) {
	t.Parallel()

	repo := &fakeSubscriptionRepo{}
	bot := &fakeBotPort{}
	poller := LinkPollingUseCase{
		logger:                 testUsecaseLogger(),
		subscriptionRepository: repo,
		botPort:                bot,
	}

	link, err := domain.NewLink("https://github.com/user/repo-updated")
	require.NoError(t, err)
	link.ID = 999
	link.LastUpdate = time.Unix(10, 0)

	expectedChatIDs := []int64{101, 202}
	newState := model.LinkState{
		UpdatedAt: time.Unix(20, 0),
	}

	poller.updateAndNotify(context.Background(), link, expectedChatIDs, newState)

	require.Len(t, repo.updatedLinks, 1)
	require.Len(t, bot.events, 1)
	gotEvent := bot.events[0]
	assert.Equal(t, link.ID, gotEvent.LinkID)
	assert.Equal(t, link.URL.String(), gotEvent.RawURL)
	assert.Equal(t, expectedChatIDs, gotEvent.ChatIDs)
}

func TestLinkPollingUseCase_UpdateAndNotify_NoChanges(t *testing.T) {
	t.Parallel()

	repo := &fakeSubscriptionRepo{}
	bot := &fakeBotPort{}
	poller := LinkPollingUseCase{
		logger:                 testUsecaseLogger(),
		subscriptionRepository: repo,
		botPort:                bot,
	}

	link, err := domain.NewLink("https://github.com/user/repo-no-changes")
	require.NoError(t, err)
	link.LastUpdate = time.Unix(20, 0)

	state := model.LinkState{
		UpdatedAt: time.Unix(20, 0),
	}
	poller.updateAndNotify(context.Background(), link, []int64{1}, state)

	assert.Empty(t, repo.updatedLinks)
	assert.Empty(t, bot.events)
}

var _ repository.SubscriptionRepository = (*fakeSubscriptionRepo)(nil)
