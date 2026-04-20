package sql_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	memstorage "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql"
)

type repoPair struct {
	chat repository.ChatRepository
	sub  repository.SubscriptionRepository
	out  repository.NotificationOutboxRepository
}

type chatLifecycleResult struct {
	DuplicateRegister string
	DeleteMissing     string
	AllChatMissing    string
}

type subscriptionScenarioResult struct {
	SubscribeMissingChat string
	MissingLookup        string
	DuplicateSubscribe   string
	MissingLinkSubs      string
	ChatSubscriptions    []subscriptionSnapshot
	LinkSubscriptions    []subscriptionSnapshot
	AllLinks             []string
	FoundLinkURL         string
	GetLinkURL           string
	Unsubscribed         subscriptionSnapshot
	UnsubscribeMissing   string
	ChatSubscriptionsEnd []subscriptionSnapshot
	LinkSubscriptionsEnd []subscriptionSnapshot
}

type subscriptionSnapshot struct {
	ChatID int64
	URL    string
	Tags   []string
}

type outboxScenarioResult struct {
	ZeroLimit          []model.OutboxLinkUpdateEvent
	InitialPending     []model.OutboxLinkUpdateEvent
	PendingFirstPage   []model.OutboxLinkUpdateEvent
	PendingAfterCursor []model.OutboxLinkUpdateEvent
	PendingAfterSent   []model.OutboxLinkUpdateEvent
	PendingAfterFailed []model.OutboxLinkUpdateEvent
	MissingMarkSent    string
	MissingMarkFailed  string
	LinkLastUpdate     time.Time
}

type SQLRepositoryIntegrationSuite struct {
	suite.Suite
	ctx       context.Context
	container testcontainers.Container
	pool      *pgxpool.Pool
}

func TestRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, &SQLRepositoryIntegrationSuite{})
}

func (s *SQLRepositoryIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	container, pool, err := startPostgresContainer(s.ctx)
	if err != nil {
		s.T().Skipf("docker is required for integration tests: %v", err)
		return
	}
	s.container = container
	s.pool = pool
	s.Require().NoError(applyMigrations(s.ctx, s.pool))
}

func (s *SQLRepositoryIntegrationSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *SQLRepositoryIntegrationSuite) SetupTest() {
	if s.pool == nil {
		s.T().Skip("postgres is not available")
	}
	_, err := s.pool.Exec(
		s.ctx,
		`TRUNCATE TABLE "notification_event", "subscription_tag", "subscription", "tag", "link", "chat" RESTART IDENTITY CASCADE;`,
	)
	s.Require().NoError(err)
}

func (s *SQLRepositoryIntegrationSuite) TestChatRepository_MatchesInMemoryBehavior() {
	expected := runChatLifecycleScenario(s.T(), newInMemoryPair())
	actual := runChatLifecycleScenario(s.T(), s.newSQLPair())
	s.Equal(expected, actual)
}

func (s *SQLRepositoryIntegrationSuite) TestSubscriptionRepository_MatchesInMemoryBehavior() {
	expected := runSubscriptionScenario(s.T(), newInMemoryPair())
	actual := runSubscriptionScenario(s.T(), s.newSQLPair())
	s.Equal(expected, actual)
}

func (s *SQLRepositoryIntegrationSuite) TestNotificationOutboxRepository_MatchesInMemoryBehavior() {
	expected := runOutboxScenario(s.T(), newInMemoryPair())
	actual := runOutboxScenario(s.T(), s.newSQLPair())
	assertOutboxScenarioEqual(s.T(), expected, actual)
}

func (s *SQLRepositoryIntegrationSuite) TestSubscriptionRepository_GetAllLinksPagination() {
	if s.pool == nil {
		s.T().Skip("postgres is not available")
	}

	repos := s.newSQLPair()
	ctx := context.Background()
	const (
		chatID    int64 = 301
		linkCount       = 25
	)

	require.NoError(s.T(), repos.chat.Register(ctx, chatID))

	expectedURLs := make([]string, 0, linkCount)
	for i := 0; i < linkCount; i++ {
		rawURL := fmt.Sprintf("https://github.com/acme/project-%02d", i)
		link := mustNewLink(s.T(), rawURL)
		expectedURLs = append(expectedURLs, rawURL)

		_, err := repos.sub.Subscribe(ctx, link, chatID, nil)
		require.NoError(s.T(), err)
	}
	sort.Strings(expectedURLs)

	allLinksCh, errCh := repos.sub.GetAllLinks(ctx)
	actualURLs := linkURLs(allLinksCh)
	errs := collectStreamErrors(errCh)

	require.Empty(s.T(), errs)
	require.Equal(s.T(), expectedURLs, actualURLs)
}

func (s *SQLRepositoryIntegrationSuite) newSQLPair() repoPair {
	return repoPair{
		chat: sqlrepo.NewChatRepository(s.pool),
		sub:  sqlrepo.NewSubscriptionRepository(s.pool, 10),
		out:  sqlrepo.NewNotificationOutboxRepository(s.pool),
	}
}

func newInMemoryPair() repoPair {
	st := memstorage.New()
	return repoPair{
		chat: st,
		sub:  st,
		out:  st,
	}
}

func runChatLifecycleScenario(t *testing.T, repos repoPair) chatLifecycleResult {
	t.Helper()

	ctx := context.Background()
	const chatID int64 = 101

	require.NoError(t, repos.chat.Register(ctx, chatID))

	result := chatLifecycleResult{
		DuplicateRegister: errorKind(repos.chat.Register(ctx, chatID)),
	}

	require.NoError(t, repos.chat.Delete(ctx, chatID))
	result.DeleteMissing = errorKind(repos.chat.Delete(ctx, chatID))

	_, err := repos.sub.AllChatSubscriptions(ctx, chatID)
	result.AllChatMissing = errorKind(err)

	return result
}

func runSubscriptionScenario(t *testing.T, repos repoPair) subscriptionScenarioResult {
	t.Helper()

	ctx := context.Background()
	const (
		chatID1 int64 = 201
		chatID2 int64 = 202
	)

	linkA := mustNewLink(t, "https://github.com/acme/project-alpha")
	linkB := mustNewLink(t, "https://stackoverflow.com/questions/12345")
	linkMissing := mustNewLink(t, "https://github.com/acme/project-missing")

	_, err := repos.sub.Subscribe(ctx, linkA, chatID1, []string{"go"})
	result := subscriptionScenarioResult{
		SubscribeMissingChat: errorKind(err),
	}

	_, err = repos.sub.FindLinkByURL(ctx, linkMissing.URL)
	result.MissingLookup = errorKind(err)

	_, err = repos.sub.AllLinkSubscriptions(ctx, linkMissing)
	result.MissingLinkSubs = errorKind(err)

	require.NoError(t, repos.chat.Register(ctx, chatID1))
	require.NoError(t, repos.chat.Register(ctx, chatID2))

	_, err = repos.sub.Subscribe(ctx, linkA, chatID1, []string{"go", "backend"})
	require.NoError(t, err)
	_, err = repos.sub.Subscribe(ctx, linkB, chatID1, []string{"daily"})
	require.NoError(t, err)
	_, err = repos.sub.Subscribe(ctx, linkA, chatID2, []string{"alerts"})
	require.NoError(t, err)

	_, err = repos.sub.Subscribe(ctx, linkA, chatID1, []string{"go", "backend"})
	result.DuplicateSubscribe = errorKind(err)

	chatSubs, err := repos.sub.AllChatSubscriptions(ctx, chatID1)
	require.NoError(t, err)
	result.ChatSubscriptions = snapshotsFromSubscriptions(t, repos.sub, chatSubs)

	linkSubs, err := repos.sub.AllLinkSubscriptions(ctx, linkA)
	require.NoError(t, err)
	result.LinkSubscriptions = snapshotsFromSubscriptions(t, repos.sub, linkSubs)

	allLinks, _ := repos.sub.GetAllLinks(ctx)
	// // will block forever
	//errs := make([]error, 0)
	//for err := range errCh {
	//	errs = append(errs, err)
	//}
	//require.Empty(t, errs)
	result.AllLinks = linkURLs(allLinks)

	foundLink, err := repos.sub.FindLinkByURL(ctx, linkA.URL)
	require.NoError(t, err)
	result.FoundLinkURL = foundLink.URL.String()

	getLink, err := repos.sub.GetLink(ctx, foundLink.ID)
	require.NoError(t, err)
	result.GetLinkURL = getLink.URL.String()

	unsubscribed, err := repos.sub.Unsubscribe(ctx, linkA, chatID1)
	require.NoError(t, err)
	result.Unsubscribed = snapshotFromSubscription(t, repos.sub, unsubscribed)

	_, err = repos.sub.Unsubscribe(ctx, linkA, chatID1)
	result.UnsubscribeMissing = errorKind(err)

	chatSubs, err = repos.sub.AllChatSubscriptions(ctx, chatID1)
	require.NoError(t, err)
	result.ChatSubscriptionsEnd = snapshotsFromSubscriptions(t, repos.sub, chatSubs)

	linkSubs, err = repos.sub.AllLinkSubscriptions(ctx, linkA)
	require.NoError(t, err)
	result.LinkSubscriptionsEnd = snapshotsFromSubscriptions(t, repos.sub, linkSubs)

	return result
}

func runOutboxScenario(t *testing.T, repos repoPair) outboxScenarioResult {
	t.Helper()

	ctx := context.Background()
	const (
		chatID1 int64 = 501
		chatID2 int64 = 502
	)

	require.NoError(t, repos.chat.Register(ctx, chatID1))
	require.NoError(t, repos.chat.Register(ctx, chatID2))

	link := mustNewLink(t, "https://github.com/acme/project-outbox")

	_, err := repos.sub.Subscribe(ctx, link, chatID1, []string{"alerts"})
	require.NoError(t, err)
	_, err = repos.sub.Subscribe(ctx, link, chatID2, []string{"daily"})
	require.NoError(t, err)

	storedLink, err := repos.sub.FindLinkByURL(ctx, link.URL)
	require.NoError(t, err)

	ts1 := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.Local)
	ts2 := ts1.Add(10 * time.Minute)
	ts3 := ts2.Add(10 * time.Minute)
	storedLink.LastUpdate = ts3

	_, err = repos.sub.UpdateLink(ctx, storedLink, []model.LinkUpdatedEvent{
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts1,
			UpdateName:   "issue-1",
			UpdateAuthor: "alice",
			Description:  "first event",
			ChatIDs:      []int64{chatID1, chatID2},
		},
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts2,
			UpdateName:   "issue-2",
			UpdateAuthor: "bob",
			Description:  "second event",
			ChatIDs:      []int64{chatID1, chatID2},
		},
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts3,
			UpdateName:   "issue-3",
			UpdateAuthor: "carol",
			Description:  "third event",
			ChatIDs:      []int64{chatID1, chatID2},
		},
	})
	require.NoError(t, err)

	result := outboxScenarioResult{}

	result.ZeroLimit, err = repos.out.GetPending(ctx, 0, 0)
	require.NoError(t, err)

	result.InitialPending, err = repos.out.GetPending(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, result.InitialPending, 3)

	result.PendingFirstPage, err = repos.out.GetPending(ctx, 0, 2)
	require.NoError(t, err)
	require.Len(t, result.PendingFirstPage, 2)

	cursor := result.PendingFirstPage[0].ID
	result.PendingAfterCursor, err = repos.out.GetPending(ctx, cursor, 10)
	require.NoError(t, err)

	require.NoError(t, repos.out.MarkSent(ctx, []int64{
		result.InitialPending[0].ID,
		result.InitialPending[2].ID,
	}))

	result.PendingAfterSent, err = repos.out.GetPending(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, result.PendingAfterSent, 1)

	require.NoError(t, repos.out.MarkFailed(ctx, result.PendingAfterSent[0].ID, "delivery failed"))

	result.PendingAfterFailed, err = repos.out.GetPending(ctx, 0, 10)
	require.NoError(t, err)

	result.MissingMarkSent = errorKind(repos.out.MarkSent(ctx, []int64{9999}))
	result.MissingMarkFailed = errorKind(repos.out.MarkFailed(ctx, 9999, "ignored"))

	updatedLink, err := repos.sub.GetLink(ctx, storedLink.ID)
	require.NoError(t, err)
	result.LinkLastUpdate = updatedLink.LastUpdate

	return result
}

func assertOutboxScenarioEqual(
	t *testing.T, expected, actual outboxScenarioResult,
) {
	t.Helper()

	assertOutboxEventsEqual(t, "zero_limit", expected.ZeroLimit, actual.ZeroLimit)
	assertOutboxEventsEqual(t, "initial_pending", expected.InitialPending, actual.InitialPending)
	assertOutboxEventsEqual(t, "pending_first_page", expected.PendingFirstPage, actual.PendingFirstPage)
	assertOutboxEventsEqual(t, "pending_after_cursor", expected.PendingAfterCursor, actual.PendingAfterCursor)
	assertOutboxEventsEqual(t, "pending_after_sent", expected.PendingAfterSent, actual.PendingAfterSent)
	assertOutboxEventsEqual(t, "pending_after_failed", expected.PendingAfterFailed, actual.PendingAfterFailed)
	require.Equal(t, expected.MissingMarkSent, actual.MissingMarkSent, "missing_mark_sent")
	require.Equal(t, expected.MissingMarkFailed, actual.MissingMarkFailed, "missing_mark_failed")
	require.Equal(t, expected.LinkLastUpdate, actual.LinkLastUpdate, "link_last_update")
}

func assertOutboxEventsEqual(
	t *testing.T, step string, expected, actual []model.OutboxLinkUpdateEvent,
) {
	t.Helper()

	require.Lenf(t, actual, len(expected), "%s: unexpected event count", step)
	for i, expectedEvent := range expected {
		actualEvent := actual[i]
		require.Equalf(t, expectedEvent.ID, actualEvent.ID, "%s[%d].id", step, i)
		require.Equalf(t, expectedEvent.LinkID, actualEvent.LinkID, "%s[%d].link_id", step, i)
		assertLinkUpdatedEventEqual(t, fmt.Sprintf("%s[%d].event", step, i), expectedEvent.Event, actualEvent.Event)
	}
}

func assertLinkUpdatedEventEqual(
	t *testing.T, path string, expected, actual model.LinkUpdatedEvent,
) {
	t.Helper()

	require.Equal(t, expected.LinkID, actual.LinkID, path+".link_id")
	require.Equal(t, expected.RawURL, actual.RawURL, path+".raw_url")
	require.Equal(t, expected.UpdatedAt, actual.UpdatedAt, path+".updated_at")
	require.Equal(t, expected.UpdateName, actual.UpdateName, path+".update_name")
	require.Equal(t, expected.UpdateAuthor, actual.UpdateAuthor, path+".update_author")
	require.Equal(t, expected.Description, actual.Description, path+".description")
	require.Equal(t, expected.ChatIDs, actual.ChatIDs, path+".chat_ids")
}

func snapshotsFromSubscriptions(
	t *testing.T, repo repository.SubscriptionRepository, subs []domain.Subscription,
) []subscriptionSnapshot {
	t.Helper()

	snapshots := make([]subscriptionSnapshot, len(subs))
	for i, sub := range subs {
		snapshots[i] = snapshotFromSubscription(t, repo, sub)
	}
	sortSubscriptionSnapshots(snapshots)
	return snapshots
}

func snapshotFromSubscription(
	t *testing.T, repo repository.SubscriptionRepository, sub domain.Subscription,
) subscriptionSnapshot {
	t.Helper()

	link, err := repo.GetLink(context.Background(), sub.LinkID)
	require.NoError(t, err)

	tags := append([]string(nil), sub.Tags...)
	sort.Strings(tags)

	return subscriptionSnapshot{
		ChatID: sub.ChatID,
		URL:    link.URL.String(),
		Tags:   tags,
	}
}

func linkURLs(links <-chan domain.Link) []string {
	urls := make([]string, 0)
	for link := range links {
		urls = append(urls, link.URL.String())
	}
	sort.Strings(urls)
	return urls
}

func collectStreamErrors(errCh <-chan error) []error {
	errs := make([]error, 0)
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func sortSubscriptionSnapshots(snapshots []subscriptionSnapshot) {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ChatID != snapshots[j].ChatID {
			return snapshots[i].ChatID < snapshots[j].ChatID
		}
		if snapshots[i].URL != snapshots[j].URL {
			return snapshots[i].URL < snapshots[j].URL
		}
		return strings.Join(snapshots[i].Tags, ",") < strings.Join(snapshots[j].Tags, ",")
	})
}

func errorKind(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, repository.ErrEntityAlreadyExists):
		return "already_exists"
	case errors.Is(err, repository.ErrEntityNotFound):
		return "not_found"
	case errors.Is(err, repository.ErrUnknownDBError):
		return "unknown_db"
	default:
		return fmt.Sprintf("unexpected:%T:%v", err, err)
	}
}

func mustNewLink(t *testing.T, rawURL string) domain.Link {
	t.Helper()

	link, err := domain.NewLink(rawURL)
	require.NoError(t, err)
	return link
}

func startPostgresContainer(
	ctx context.Context,
) (container testcontainers.Container, pool *pgxpool.Pool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("start postgres container: %v", recovered)
		}
	}()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "link_tracker_test",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(2 * time.Minute),
	}

	container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, err
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, err
	}

	dsn := fmt.Sprintf(
		"postgres://postgres:postgres@%s:%s/link_tracker_test?sslmode=disable",
		host,
		port.Port(),
	)
	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		if err = pool.Ping(pingCtx); err == nil {
			break
		}
		if pingCtx.Err() != nil {
			pool.Close()
			_ = container.Terminate(ctx)
			return nil, nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}

	return container, pool, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, filepath.Join(repoRoot(), "migrations"))
}

func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../"))
}
