package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type fakeSubscriptionRepo struct {
	updatedLinks            []domain.Link
	notificationOutbox      [][]model.LinkUpdatedEvent
	updateLinkErr           error
	allLinkSubscriptions    map[int64][]domain.Subscription
	allLinkSubscriptionsErr map[int64]error
}

func (f *fakeSubscriptionRepo) Subscribe(
	_ context.Context, _ domain.Link, _ int64, _ []string,
) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}

func (f *fakeSubscriptionRepo) Unsubscribe(
	_ context.Context, _ domain.Link, _ int64,
) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}

func (f *fakeSubscriptionRepo) GetLink(
	_ context.Context, _ int64,
) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *fakeSubscriptionRepo) GetAllLinks(
	_ context.Context,
) (<-chan domain.Link, <-chan error) {
	return nil, nil
}

func (f *fakeSubscriptionRepo) UpdateLink(
	_ context.Context, link domain.Link, updates []model.LinkUpdatedEvent,
) (domain.Link, error) {
	f.updatedLinks = append(f.updatedLinks, link)
	f.notificationOutbox = append(f.notificationOutbox, clonePollingEvents(updates))
	if f.updateLinkErr != nil {
		return domain.Link{}, f.updateLinkErr
	}
	return link, nil
}

func (f *fakeSubscriptionRepo) FindLinkByURL(
	_ context.Context, _ *url.URL,
) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *fakeSubscriptionRepo) AllChatSubscriptions(
	_ context.Context, _ int64,
) ([]domain.Subscription, error) {
	return nil, nil
}

func (f *fakeSubscriptionRepo) AllLinkSubscriptions(
	_ context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	if err, ok := f.allLinkSubscriptionsErr[link.ID]; ok {
		return nil, err
	}
	subs := f.allLinkSubscriptions[link.ID]
	return append([]domain.Subscription(nil), subs...), nil
}

type fakeLinkStatePort struct {
	updatesByLinkID map[int64][]model.LinkState
	errsByLinkID    map[int64]error
	fetchCalls      []domain.Link
}

func (f *fakeLinkStatePort) Fetch(
	_ context.Context, link domain.Link,
) ([]model.LinkState, error) {
	f.fetchCalls = append(f.fetchCalls, link)
	if err, ok := f.errsByLinkID[link.ID]; ok {
		return nil, err
	}
	updates := f.updatesByLinkID[link.ID]
	return append([]model.LinkState(nil), updates...), nil
}

func testUseCaseLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLinkPollingUseCase_Start_CreatesJob(t *testing.T) {
	t.Parallel()

	sched, err := gocron.NewScheduler()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, sched.Shutdown())
	}()

	uc := NewLinkPollingUseCase(
		testUseCaseLogger(),
		sched,
		&fakeLinkStatePort{},
		&fakeSubscriptionRepo{},
		15,
	)

	job, err := uc.Start(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, job)
}

func TestLinkPollingUseCase_UpdateAndNotify_NewerUpdates(t *testing.T) {
	t.Parallel()

	repo := &fakeSubscriptionRepo{}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		subscriptionRepository: repo,
	}

	link := mustNewPollingLink(t, "https://github.com/user/repo-updated")
	link.ID = 999
	link.LastUpdate = time.Unix(10, 0)

	expectedChatIDs := []int64{101, 202}
	update1 := model.LinkState{
		UpdatedAt:  time.Unix(20, 0),
		UpdatedBy:  "alice",
		Title:      "issue-1",
		UpdateInfo: "first change",
	}
	update2 := model.LinkState{
		UpdatedAt:  time.Unix(21, 0),
		UpdatedBy:  "bob",
		Title:      "issue-2",
		UpdateInfo: "second change",
	}

	poller.updateAndNotify(
		context.Background(), link, expectedChatIDs,
		[]model.LinkState{update1, update2},
	)

	require.Len(t, repo.updatedLinks, 1)
	require.Len(t, repo.notificationOutbox, 1)
	assert.Equal(t, time.Unix(21, 0), repo.updatedLinks[0].LastUpdate)
	assert.Equal(t, []model.LinkUpdatedEvent{
		{
			ChatIDs:      expectedChatIDs,
			UpdatedAt:    update1.UpdatedAt,
			UpdateAuthor: update1.UpdatedBy,
			UpdateName:   update1.Title,
			LinkID:       link.ID,
			RawURL:       link.URL.String(),
			Description:  update1.UpdateInfo,
		},
		{
			ChatIDs:      expectedChatIDs,
			UpdatedAt:    update2.UpdatedAt,
			UpdateAuthor: update2.UpdatedBy,
			UpdateName:   update2.Title,
			LinkID:       link.ID,
			RawURL:       link.URL.String(),
			Description:  update2.UpdateInfo,
		},
	}, repo.notificationOutbox[0])
}

func TestLinkPollingUseCase_UpdateAndNotify_NoChanges(t *testing.T) {
	t.Parallel()

	repo := &fakeSubscriptionRepo{}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		subscriptionRepository: repo,
	}

	link := mustNewPollingLink(t, "https://github.com/user/repo-no-changes")
	link.LastUpdate = time.Unix(20, 0)

	state := model.LinkState{
		UpdatedAt: time.Unix(20, 0),
	}
	poller.updateAndNotify(context.Background(), link, []int64{1}, []model.LinkState{state})

	assert.Empty(t, repo.updatedLinks)
	assert.Empty(t, repo.notificationOutbox)
}

func TestLinkPollingUseCase_UpdateAndNotify_UpdateLinkErrorIsIgnored(t *testing.T) {
	t.Parallel()

	repo := &fakeSubscriptionRepo{
		updateLinkErr: errors.New("can't persist outbox"),
	}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		subscriptionRepository: repo,
	}

	link := mustNewPollingLink(t, "https://github.com/user/repo-update-error")
	link.ID = 777
	link.LastUpdate = time.Unix(10, 0)

	poller.updateAndNotify(context.Background(), link, []int64{42}, []model.LinkState{{
		UpdatedAt:  time.Unix(30, 0),
		UpdatedBy:  "alice",
		Title:      "issue-1",
		UpdateInfo: "changed",
	}})

	require.Len(t, repo.updatedLinks, 1)
	require.Len(t, repo.notificationOutbox, 1)
	assert.Equal(t, time.Unix(30, 0), repo.updatedLinks[0].LastUpdate)
}

func TestLinkPollingUseCase_LinkWorkerFn_UsesFetchedUpdatesAndSubscriptions(t *testing.T) {
	t.Parallel()

	link := mustNewPollingLink(t, "https://github.com/user/repo-worker")
	link.ID = 301
	link.LastUpdate = time.Unix(10, 0)

	repo := &fakeSubscriptionRepo{
		allLinkSubscriptions: map[int64][]domain.Subscription{
			link.ID: {
				domain.NewSubscription(1, link.ID, 101, nil),
				domain.NewSubscription(2, link.ID, 202, nil),
			},
		},
	}
	linkState := &fakeLinkStatePort{
		updatesByLinkID: map[int64][]model.LinkState{
			link.ID: {
				{
					UpdatedAt:  time.Unix(20, 0),
					UpdatedBy:  "alice",
					Title:      "issue-1",
					UpdateInfo: "first change",
				},
				{
					UpdatedAt:  time.Unix(21, 0),
					UpdatedBy:  "bob",
					Title:      "issue-2",
					UpdateInfo: "second change",
				},
			},
		},
	}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		linkState:              linkState,
		subscriptionRepository: repo,
	}

	poller.linkWorkerFn(context.Background(), singleLinkChan(link))

	require.Len(t, linkState.fetchCalls, 1)
	require.Equal(t, link.ID, linkState.fetchCalls[0].ID)
	require.Len(t, repo.updatedLinks, 1)
	require.Len(t, repo.notificationOutbox, 1)
	assert.Equal(t, []int64{101, 202}, repo.notificationOutbox[0][0].ChatIDs)
	assert.Equal(t, []int64{101, 202}, repo.notificationOutbox[0][1].ChatIDs)
	assert.Equal(t, time.Unix(21, 0), repo.updatedLinks[0].LastUpdate)
}

func TestLinkPollingUseCase_LinkWorkerFn_SkipsWhenSubscriptionsFail(t *testing.T) {
	t.Parallel()

	link := mustNewPollingLink(t, "https://github.com/user/repo-subscriptions-error")
	link.ID = 302

	repo := &fakeSubscriptionRepo{
		allLinkSubscriptionsErr: map[int64]error{
			link.ID: errors.New("can't fetch subscriptions"),
		},
	}
	linkState := &fakeLinkStatePort{}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		linkState:              linkState,
		subscriptionRepository: repo,
	}

	poller.linkWorkerFn(context.Background(), singleLinkChan(link))

	assert.Empty(t, linkState.fetchCalls)
	assert.Empty(t, repo.updatedLinks)
	assert.Empty(t, repo.notificationOutbox)
}

func TestLinkPollingUseCase_LinkWorkerFn_CreatesFallbackUpdateWhenFetchFails(t *testing.T) {
	t.Parallel()

	link := mustNewPollingLink(t, "https://github.com/user/repo-fetch-error")
	link.ID = 303
	link.LastUpdate = time.Unix(10, 0)

	repo := &fakeSubscriptionRepo{
		allLinkSubscriptions: map[int64][]domain.Subscription{
			link.ID: {
				domain.NewSubscription(1, link.ID, 101, nil),
				domain.NewSubscription(2, link.ID, 202, nil),
			},
		},
	}
	linkState := &fakeLinkStatePort{
		errsByLinkID: map[int64]error{
			link.ID: errors.New("boom"),
		},
	}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		linkState:              linkState,
		subscriptionRepository: repo,
	}

	startedAt := time.Now()
	poller.linkWorkerFn(context.Background(), singleLinkChan(link))
	finishedAt := time.Now()

	require.Len(t, linkState.fetchCalls, 1)
	require.Len(t, repo.updatedLinks, 1)
	require.Len(t, repo.notificationOutbox, 1)
	require.Len(t, repo.notificationOutbox[0], 1)

	update := repo.notificationOutbox[0][0]
	assert.Equal(t, "Could not fetch link state", update.UpdateName)
	assert.Equal(t, "-", update.UpdateAuthor)
	assert.Equal(t, "Cause: boom", update.Description)
	assert.Equal(t, []int64{101, 202}, update.ChatIDs)
	assert.False(t, update.UpdatedAt.Before(startedAt))
	assert.False(t, update.UpdatedAt.After(finishedAt))
	assert.Equal(t, update.UpdatedAt, repo.updatedLinks[0].LastUpdate)
}

func TestLinkPollingUseCase_LinkWorkerFn_SkipsWhenFetchReturnsNoUpdates(t *testing.T) {
	t.Parallel()

	link := mustNewPollingLink(t, "https://github.com/user/repo-no-updates")
	link.ID = 304

	repo := &fakeSubscriptionRepo{
		allLinkSubscriptions: map[int64][]domain.Subscription{
			link.ID: {domain.NewSubscription(1, link.ID, 101, nil)},
		},
	}
	linkState := &fakeLinkStatePort{
		updatesByLinkID: map[int64][]model.LinkState{
			link.ID: {},
		},
	}
	poller := LinkPollingUseCase{
		logger:                 testUseCaseLogger(),
		linkState:              linkState,
		subscriptionRepository: repo,
	}

	poller.linkWorkerFn(context.Background(), singleLinkChan(link))

	require.Len(t, linkState.fetchCalls, 1)
	assert.Empty(t, repo.updatedLinks)
	assert.Empty(t, repo.notificationOutbox)
}

func mustNewPollingLink(t *testing.T, rawURL string) domain.Link {
	t.Helper()

	link, err := domain.NewLink(rawURL)
	require.NoError(t, err)
	return link
}

func singleLinkChan(link domain.Link) <-chan domain.Link {
	ch := make(chan domain.Link, 1)
	ch <- link
	close(ch)
	return ch
}

func clonePollingEvents(events []model.LinkUpdatedEvent) []model.LinkUpdatedEvent {
	cloned := make([]model.LinkUpdatedEvent, len(events))
	for i, event := range events {
		event.ChatIDs = append([]int64(nil), event.ChatIDs...)
		cloned[i] = event
	}
	return cloned
}

var _ repository.SubscriptionRepository = (*fakeSubscriptionRepo)(nil)
var _ ports.LinkStatePort = (*fakeLinkStatePort)(nil)
