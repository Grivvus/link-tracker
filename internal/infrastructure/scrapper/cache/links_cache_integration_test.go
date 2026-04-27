//go:build integrational

package cache_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/cache"
)

const (
	valkeyTestImage    = "valkey/valkey:9.0"
	valkeyTestPort     = "6379/tcp"
	valkeyTestPassword = "test-password"
	defaultCacheTTL    = time.Minute
)

type LinksCacheIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	container   testcontainers.Container
	adminClient valkey.Client
	testClients []valkey.Client
	address     string
}

func TestLinksCacheIntegrationSuite(t *testing.T) {
	suite.Run(t, new(LinksCacheIntegrationSuite))
}

func (s *LinksCacheIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	container, adminClient, address, err := startValkeyContainer(s.ctx)
	if err != nil {
		s.T().Skipf("docker is required for cache integration tests: %v", err)
		return
	}

	s.container = container
	s.adminClient = adminClient
	s.address = address
}

func (s *LinksCacheIntegrationSuite) TearDownSuite() {
	if s.adminClient != nil {
		s.adminClient.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *LinksCacheIntegrationSuite) SetupTest() {
	if s.adminClient == nil {
		s.T().Skip("valkey is not available")
	}
	s.FlushCache()
}

func (s *LinksCacheIntegrationSuite) TearDownTest() {
	for _, client := range s.testClients {
		client.Close()
	}
	s.testClients = nil
}

func (s *LinksCacheIntegrationSuite) NewLinksCache(ttl time.Duration) cache.LinksCache {
	if ttl == 0 {
		ttl = defaultCacheTTL
	}
	return cache.NewValkeyLinksCache(discardLogger(), s.NewValkeyClient(), ttl)
}

func (s *LinksCacheIntegrationSuite) NewValkeyClient() valkey.Client {
	client, err := newValkeyClient(s.address)
	s.Require().NoError(err)
	s.testClients = append(s.testClients, client)
	return client
}

func (s *LinksCacheIntegrationSuite) ValkeyAddress() string {
	return s.address
}

func (s *LinksCacheIntegrationSuite) FlushCache() {
	err := s.adminClient.Do(s.ctx, s.adminClient.B().Flushdb().Build()).Error()
	s.Require().NoError(err)
}

func (s *LinksCacheIntegrationSuite) TestSetThenGetReturnsSameLinks() {
	const chatID int64 = 101
	expected := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: "https://github.com/valkey-io/valkey",
			Tags:   []string{"cache", "go"},
		},
		{
			LinkID: 2,
			RawURL: "https://stackoverflow.com/questions/123",
			Tags:   []string{"stackoverflow"},
		},
	}
	linksCache := s.NewLinksCache(time.Minute)

	s.Require().NoError(linksCache.Set(s.ctx, chatID, expected))

	actual, hit, err := linksCache.Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.True(hit)
	s.Equal(expected, actual)
}

func (s *LinksCacheIntegrationSuite) TestDeleteRemovesCachedLinks() {
	const chatID int64 = 102
	linksCache := s.NewLinksCache(time.Minute)
	links := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: "https://github.com/valkey-io/valkey-go",
			Tags:   []string{"delete"},
		},
	}

	s.Require().NoError(linksCache.Set(s.ctx, chatID, links))
	actual, hit, err := s.NewLinksCache(time.Minute).Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.True(hit)
	s.Equal(links, actual)

	s.Require().NoError(linksCache.Delete(s.ctx, chatID))

	actual, hit, err = s.NewLinksCache(time.Minute).Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.False(hit)
	s.Nil(actual)
}

func (s *LinksCacheIntegrationSuite) TestTTLExpiresCachedLinks() {
	const chatID int64 = 103
	const ttl = 250 * time.Millisecond
	const expirationGrace = 100 * time.Millisecond
	linksCache := s.NewLinksCache(ttl)
	links := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: "https://github.com/valkey-io/valkey",
			Tags:   []string{"ttl"},
		},
	}

	startedAt := time.Now()
	s.Require().NoError(linksCache.Set(s.ctx, chatID, links))

	actual, hit, err := linksCache.Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.True(hit)
	s.Equal(links, actual)

	time.Sleep(ttl / 3)
	actual, hit, err = linksCache.Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.True(hit)
	s.Equal(links, actual)

	if waitUntilExpiration := ttl + expirationGrace - time.Since(startedAt); waitUntilExpiration > 0 {
		time.Sleep(waitUntilExpiration)
	}

	actual, hit, err = linksCache.Get(s.ctx, chatID)
	s.Require().NoError(err)
	s.False(hit)
	s.Nil(actual)
}

func startValkeyContainer(
	ctx context.Context,
) (container testcontainers.Container, client valkey.Client, address string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("start valkey container: %v", recovered)
		}
	}()

	req := testcontainers.ContainerRequest{
		Image:        valkeyTestImage,
		ExposedPorts: []string{valkeyTestPort},
		Cmd: []string{
			"valkey-server",
			"--port", "6379",
			"--requirepass", valkeyTestPassword,
		},
		WaitingFor: wait.ForListeningPort(valkeyTestPort).
			WithStartupTimeout(2 * time.Minute).
			WithPollInterval(time.Second),
	}

	container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, "", err
	}
	port, err := container.MappedPort(ctx, valkeyTestPort)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, "", err
	}

	address = fmt.Sprintf("%s:%s", host, port.Port())
	client, err = newValkeyClient(address)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, "", err
	}

	if err = waitValkeyReady(ctx, client); err != nil {
		client.Close()
		_ = container.Terminate(ctx)
		return nil, nil, "", err
	}

	return container, client, address, nil
}

func newValkeyClient(address string) (valkey.Client, error) {
	return valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{address},
		Password:          valkeyTestPassword,
		ForceSingleClient: true,
	})
}

func waitValkeyReady(ctx context.Context, client valkey.Client) error {
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var lastErr error
	for {
		lastErr = client.Do(pingCtx, client.B().Ping().Build()).Error()
		if lastErr == nil {
			return nil
		}
		if pingCtx.Err() != nil {
			return lastErr
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
