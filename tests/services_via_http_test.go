package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ServicesHTTPTestSuite struct {
	suite.Suite
	ctx                context.Context
	net                *testcontainers.DockerNetwork
	botService         testcontainers.Container
	scrapperService    testcontainers.Container
	botServiceURL      func() string
	scrapperServiceURL func() string
}

func (s *ServicesHTTPTestSuite) SetupSuite() {
	ctx := context.Background()
	s.ctx = ctx
	net, err := network.New(ctx, network.WithAttachable())
	s.Require().NoError(err)
	s.net = net
	botReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile.bot",
		},
		Env: map[string]string{
			"BOT_TOKEN":                        "change me",
			"BOT_URL":                          "http://bot:8000",
			"SCRAPPER_URL":                     "scrapper:8000",
			"SERVER_URL":                       "0.0.0.0:8000",
			"BOT_HOST":                         "bot",
			"SCRAPPER_HOST":                    "scrapper",
			"SCRAPPER_GRPC_PORT":               "50051",
			"BOT_GRPC_PORT":                    "50051",
			"USE_GRPC":                         "true",
			"POLLER_INTERVAL_MINUTES":          "30",
			"DB_CONN_STRING":                   "postgres://postgres:hackme@db:5432/postgres",
			"DB_ACCESS_TYPE":                   "SQL",
			"CONCURRENCY_BOUNDARY":             "8",
			"DB_LINKS_PAGE_SIZE":               "50",
			"NOTIFICATION_METHOD":              "http",
			"KAFKA_BROKERS":                    "broker-1:19092,broker-2:19092,broker-3:19092",
			"KAFKA_TOPIC":                      "link.raw-updates",
			"KAFKA_DLQ_TOPIC":                  "link.raw-updates-dlq",
			"KAFKA_PROCESSED_TOPIC":            "link.processed-update",
			"KAFKA_MAX_ATTEMPTS":               "5",
			"SCHEMA_REGISTRY_URL":              "http://kafka-registry:8081",
			"VALKEY_CLUSTER_ADDRESSES":         "valkey-0:6379,valkey-1:6379,valkey-2:6379,valkey-3:6379,valkey-4:6379,valkey-5:6379",
			"VALKEY_PASSWORD":                  "hackme",
			"VALKEY_TTL_SECONDS":               "60",
			"EXTERNAL_SERVICES_TIMEOUT_SECOND": "5",
			"RETRY_POLICY":                     "5",
			"RETRY_DELAY_MS":                   "100",
			"RATE_LIMITER_MAX_BURST":           "40",
			"RATE_LIMITER_REQUESTS_PER_SECOND": "5",
			"BREAKER_HALFOPEN_MAX_REQUESTS":    "3",
			"BREAKER_TIMEOUT_SECOND":           "30",
			"BREAKER_BUCKET_PERIOD_SECOND":     "5",
			"BREAKER_INTERVAL_SECOND":          "15",
			"BREAKER_FAILURE_TRESHOLD":         "50",
		},
		Networks:     []string{net.Name},
		ExposedPorts: []string{"8000"},
		WaitingFor:   wait.ForListeningPort("8000"),
	}

	scrapperReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile.scrapper",
		},
		Env: map[string]string{
			"BOT_TOKEN":                        "token123",
			"BOT_URL":                          "http://bot:8000",
			"SCRAPPER_URL":                     "scrapper:8000",
			"SERVER_URL":                       "0.0.0.0:8000",
			"BOT_HOST":                         "bot",
			"SCRAPPER_HOST":                    "scrapper",
			"SCRAPPER_GRPC_PORT":               "50051",
			"BOT_GRPC_PORT":                    "50051",
			"USE_GRPC":                         "true",
			"POLLER_INTERVAL_MINUTES":          "30",
			"DB_CONN_STRING":                   "postgres://postgres:hackme@db:5432/postgres",
			"DB_ACCESS_TYPE":                   "SQL",
			"CONCURRENCY_BOUNDARY":             "8",
			"DB_LINKS_PAGE_SIZE":               "50",
			"NOTIFICATION_METHOD":              "http",
			"KAFKA_BROKERS":                    "broker-1:19092,broker-2:19092,broker-3:19092",
			"KAFKA_TOPIC":                      "notifications",
			"KAFKA_DLQ_TOPIC":                  "notifications-dlq",
			"KAFKA_MAX_ATTEMPTS":               "5",
			"SCHEMA_REGISTRY_URL":              "http://kafka-registry:8081",
			"VALKEY_CLUSTER_ADDRESSES":         "valkey-0:6379,valkey-1:6379,valkey-2:6379,valkey-3:6379,valkey-4:6379,valkey-5:6379",
			"VALKEY_PASSWORD":                  "hackme",
			"VALKEY_TTL_SECONDS":               "60",
			"EXTERNAL_SERVICES_TIMEOUT_SECOND": "5",
			"RETRY_POLICY":                     "5",
			"RETRY_DELAY_MS":                   "100",
			"RATE_LIMITER_MAX_BURST":           "40",
			"RATE_LIMITER_REQUESTS_PER_SECOND": "5",
			"BREAKER_HALFOPEN_MAX_REQUESTS":    "3",
			"BREAKER_TIMEOUT_SECOND":           "30",
			"BREAKER_BUCKET_PERIOD_SECOND":     "5",
			"BREAKER_INTERVAL_SECOND":          "15",
			"BREAKER_FAILURE_TRESHOLD":         "50",
		},
		Networks:     []string{net.Name},
		ExposedPorts: []string{"8000"},
		WaitingFor:   wait.ForListeningPort("8000"),
	}

	botService, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: botReq,
		Started:          true,
	})
	s.Require().NoError(err)
	scrapperService, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrapperReq,
		Started:          true,
	})
	s.Require().NoError(err)
	s.botService = botService
	s.scrapperService = scrapperService
	s.botServiceURL = func() string {
		botHost, err := botService.Host(ctx)
		botPort, err1 := botService.MappedPort(ctx, "8000/tcp")
		s.Require().NoError(err)
		s.Require().NoError(err1)
		return strings.TrimSuffix(fmt.Sprintf("http://%v:%v", botHost, botPort), "/tcp")
	}
	s.scrapperServiceURL = func() string {
		scrapperHost, err := scrapperService.Host(ctx)
		scrapperPort, err1 := scrapperService.MappedPort(ctx, "8000/tcp")
		s.Require().NoError(err)
		s.Require().NoError(err1)
		return strings.TrimSuffix(fmt.Sprintf("http://%v:%v", scrapperHost, scrapperPort), "/tcp")
	}
}

func (s *ServicesHTTPTestSuite) TearDownSuite() {
	s.net.Remove(s.ctx)
	s.botService.Terminate(s.ctx)
	s.scrapperService.Terminate(s.ctx)
}

func (s *ServicesHTTPTestSuite) TestTgChat_RegisterAndDelete() {
	chatID := s.nextChatID()
	resp := s.registerChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")

	resp = s.deleteChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be deleted successfully")
}

func (s *ServicesHTTPTestSuite) TestTgChat_RegisterConflict() {
	chatID := s.nextChatID()
	resp := s.registerChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")

	resp = s.registerChat(chatID)
	s.Equal(http.StatusConflict, resp.StatusCode, "chat already exists: expected 409")

	resp = s.deleteChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be deleted successfully")
}

func (s *ServicesHTTPTestSuite) TestTgChat_DeleteNotFound() {
	chatID := s.nextChatID()
	resp := s.deleteChat(chatID)
	s.Equal(http.StatusNotFound, resp.StatusCode, "chat not registered: expected 404")
}

func (s *ServicesHTTPTestSuite) TestLinks_GetRequiresHeader() {
	resp, _ := s.doRequest(http.MethodGet, "/links", nil, nil)
	s.Equal(http.StatusBadRequest, resp.StatusCode, "missing Tg-Chat-Id header: expected 400")
}

func (s *ServicesHTTPTestSuite) TestLinks_ListAfterChatRegistration() {
	chatID := s.nextChatID()
	resp := s.registerChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")
	defer func() {
		resp := s.deleteChat(chatID)
		s.Equal(http.StatusOK, resp.StatusCode, "chat should be deleted successfully")
	}()

	resp, body := s.getLinks(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links is OK for registered chat")
	var parsed listLinksResponse
	err := json.Unmarshal(body, &parsed)
	s.Require().NoError(err)
	s.Equal(int32(0), parsed.Size, "expected no links for new chat")
	s.Empty(parsed.Links, "expected empty links list for new chat")
}

func (s *ServicesHTTPTestSuite) TestLinks_AddAndDelete() {
	chatID := s.nextChatID()
	resp := s.registerChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")
	defer func() {
		resp := s.deleteChat(chatID)
		s.Equal(http.StatusOK, resp.StatusCode, "chat should be deleted successfully")
	}()

	link := fmt.Sprintf("https://stackoverflow.com/questions/%d", chatID)
	tags := []string{"t1", "t2"}

	resp, body := s.addLink(chatID, link, tags)
	s.Equal(http.StatusOK, resp.StatusCode, "link should be added successfully")
	var added linkResponse
	err := json.Unmarshal(body, &added)
	s.Require().NoError(err)
	s.Equal(link, added.URL, "added link url should match")
	s.ElementsMatch(tags, added.Tags, "tags should match")
	s.Positive(added.ID, "link id should be set")

	resp, _ = s.addLink(chatID, link, tags)
	s.Equal(http.StatusConflict, resp.StatusCode, "duplicate link should return 409")

	resp, body = s.getLinks(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links is OK for registered chat")
	var listed listLinksResponse
	err = json.Unmarshal(body, &listed)
	s.Require().NoError(err)
	s.Require().Equal(int32(1), listed.Size, "expected one tracked link")
	s.Require().Len(listed.Links, 1, "expected one tracked link in response")
	s.Equal(link, listed.Links[0].URL, "listed link url should match")

	resp, body = s.deleteLink(chatID, link)
	s.Equal(http.StatusOK, resp.StatusCode, "link should be deleted successfully")
	var deleted linkResponse
	err = json.Unmarshal(body, &deleted)
	s.Require().NoError(err)
	s.Equal(link, deleted.URL, "deleted link url should match")

	resp, body = s.getLinks(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links is OK after delete")
	err = json.Unmarshal(body, &listed)
	s.Require().NoError(err)
	s.Equal(int32(0), listed.Size, "expected no links after delete")
	s.Empty(listed.Links, "expected empty links list after delete")
}

func (s *ServicesHTTPTestSuite) TestLinks_AddLinkWithUnsupportedHost() {
	chatID := s.nextChatID()
	resp := s.registerChat(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")
	defer func() {
		resp := s.deleteChat(chatID)
		s.Equal(http.StatusOK, resp.StatusCode, "chat should be deleted successfully")
	}()

	link := fmt.Sprintf("https://google.com/questions/%d", chatID)
	tags := []string{"t1", "t2"}

	resp, _ = s.addLink(chatID, link, tags)
	s.Equal(http.StatusBadRequest, resp.StatusCode, "unsupported host: expected StatusCode 400")
}

func (s *ServicesHTTPTestSuite) TestLinks_AddFailsIfNotRegistered() {
	chatID := s.nextChatID()
	link := fmt.Sprintf("https://stackoverflow.com/questions/%d", chatID)
	resp, _ := s.addLink(chatID, link, []string{"t1"})
	s.Equal(http.StatusNotFound, resp.StatusCode, "chat not registered: expected 404")
}

func (s *ServicesHTTPTestSuite) TestLinks_DeleteRequiresHeader() {
	payload, err := json.Marshal(map[string]any{"link": "https://stackoverflow.com/questions/1"})
	s.Require().NoError(err)
	resp, _ := s.doRequest(http.MethodDelete, "/links", payload, nil)
	s.Equal(http.StatusBadRequest, resp.StatusCode, "missing Tg-Chat-Id header: expected 400")
}

func (s *ServicesHTTPTestSuite) nextChatID() int64 {
	return atomic.AddInt64(&chatIDCounter, 1)
}

func (s *ServicesHTTPTestSuite) doRequest(
	method string,
	path string,
	body []byte,
	headers map[string]string,
) (*http.Response, []byte) {
	bodyReader := bytes.NewReader(body)
	req, err := http.NewRequest(method, s.scrapperServiceURL()+path, bodyReader)
	s.Require().NoError(err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, respBody
}

func (s *ServicesHTTPTestSuite) registerChat(chatID int64) *http.Response {
	resp, _ := s.doRequest(
		http.MethodPost,
		fmt.Sprintf("/tg-chat/%d", chatID),
		nil,
		nil,
	)
	return resp
}

func (s *ServicesHTTPTestSuite) deleteChat(chatID int64) *http.Response {
	resp, _ := s.doRequest(
		http.MethodDelete,
		fmt.Sprintf("/tg-chat/%d", chatID),
		nil,
		nil,
	)
	return resp
}

func (s *ServicesHTTPTestSuite) addLink(chatID int64, link string, tags []string) (*http.Response, []byte) {
	payload, err := json.Marshal(map[string]any{
		"link": link,
		"tags": tags,
	})
	s.Require().NoError(err)
	resp, body := s.doRequest(
		http.MethodPost,
		"/links",
		payload,
		map[string]string{"Tg-Chat-Id": strconv.FormatInt(chatID, 10)},
	)
	return resp, body
}
func (s *ServicesHTTPTestSuite) getLinks(chatID int64) (*http.Response, []byte) {
	resp, body := s.doRequest(
		http.MethodGet,
		"/links",
		nil,
		map[string]string{"Tg-Chat-Id": strconv.FormatInt(chatID, 10)},
	)
	return resp, body
}

func (s *ServicesHTTPTestSuite) deleteLink(chatID int64, link string) (*http.Response, []byte) {
	payload, err := json.Marshal(map[string]any{
		"link": link,
	})
	s.Require().NoError(err)
	resp, body := s.doRequest(
		http.MethodDelete,
		"/links",
		payload,
		map[string]string{"Tg-Chat-Id": strconv.FormatInt(chatID, 10)},
	)
	return resp, body
}

func TestSuiteStart(t *testing.T) {
	suite.Run(t, &ServicesHTTPTestSuite{})
}
