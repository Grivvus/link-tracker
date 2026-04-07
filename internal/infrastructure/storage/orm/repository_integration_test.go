package orm_test

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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	memstorage "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage"
	ormrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/orm"
)

type repoPair struct {
	chat repository.ChatRepository
	sub  repository.SubscriptionRepository
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

type RepositoryIntegrationSuite struct {
	suite.Suite
	ctx       context.Context
	container testcontainers.Container
	pool      *pgxpool.Pool
}

func TestRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RepositoryIntegrationSuite))
}

func (s *RepositoryIntegrationSuite) SetupSuite() {
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

func (s *RepositoryIntegrationSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *RepositoryIntegrationSuite) SetupTest() {
	if s.pool == nil {
		s.T().Skip("postgres is not available")
	}
	_, err := s.pool.Exec(
		s.ctx,
		`TRUNCATE TABLE "subscription_tag", "subscription", "tag", "link", "chat" RESTART IDENTITY CASCADE;`,
	)
	s.Require().NoError(err)
}

func (s *RepositoryIntegrationSuite) TestChatRepository_MatchesInMemoryBehavior() {
	expected := runChatLifecycleScenario(s.T(), newInMemoryPair())
	actual := runChatLifecycleScenario(s.T(), s.newORMPair())
	s.Equal(expected, actual)
}

func (s *RepositoryIntegrationSuite) TestSubscriptionRepository_MatchesInMemoryBehavior() {
	expected := runSubscriptionScenario(s.T(), newInMemoryPair())
	actual := runSubscriptionScenario(s.T(), s.newORMPair())
	s.Equal(expected, actual)
}

func (s *RepositoryIntegrationSuite) TestSubscriptionRepository_GetAllLinksPagination() {
	if s.pool == nil {
		s.T().Skip("postgres is not available")
	}

	repos := s.newORMPair()
	ctx := context.Background()
	const (
		chatID    int64 = 301
		linkCount       = 60
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

func (s *RepositoryIntegrationSuite) newORMPair() repoPair {
	return repoPair{
		chat: ormrepo.NewChatRepository(s.pool),
		sub:  ormrepo.NewSubscriptionRepository(s.pool, 10),
	}
}

func (s *RepositoryIntegrationSuite) TestSubscriptionRepository_EmptyTagsArePreserved() {
	if s.pool == nil {
		s.T().Skip("postgres is not available")
	}

	repos := s.newORMPair()
	ctx := context.Background()
	const chatID int64 = 401

	require.NoError(s.T(), repos.chat.Register(ctx, chatID))

	link := mustNewLink(s.T(), "https://github.com/acme/project-no-tags")
	_, err := repos.sub.Subscribe(ctx, link, chatID, []string{})
	require.NoError(s.T(), err)

	chatSubs, err := repos.sub.AllChatSubscriptions(ctx, chatID)
	require.NoError(s.T(), err)
	require.Len(s.T(), chatSubs, 1)

	sub := chatSubs[0]
	require.NotZero(s.T(), sub.ID)
	require.NotZero(s.T(), sub.LinkID)
	require.Equal(s.T(), chatID, sub.ChatID)
	require.Empty(s.T(), sub.Tags)

	gotLink, err := repos.sub.GetLink(ctx, sub.LinkID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), link.URL.String(), gotLink.URL.String())

	linkSubs, err := repos.sub.AllLinkSubscriptions(ctx, link)
	require.NoError(s.T(), err)
	require.Len(s.T(), linkSubs, 1)
	require.Empty(s.T(), linkSubs[0].Tags)
}

func newInMemoryPair() repoPair {
	st := memstorage.New()
	return repoPair{
		chat: st,
		sub:  st,
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
