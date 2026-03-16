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
	s.NoError(err)
	s.net = net
	botReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile.bot",
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
		Networks:     []string{net.Name},
		ExposedPorts: []string{"8000"},
		WaitingFor:   wait.ForListeningPort("8000"),
	}

	botService, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: botReq,
		Started:          true,
	})
	s.NoError(err)
	scrapperService, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrapperReq,
		Started:          true,
	})
	s.NoError(err)
	s.botService = botService
	s.scrapperService = scrapperService
	s.botServiceURL = func() string {
		botHost, err := botService.Host(ctx)
		botPort, err1 := botService.MappedPort(ctx, "8000/tcp")
		s.NoError(err)
		s.NoError(err1)
		return strings.TrimSuffix(fmt.Sprintf("http://%v:%v", botHost, botPort), "/tcp")
	}
	s.scrapperServiceURL = func() string {
		scrapperHost, err := scrapperService.Host(ctx)
		scrapperPort, err1 := scrapperService.MappedPort(ctx, "8000/tcp")
		s.NoError(err)
		s.NoError(err1)
		return strings.TrimSuffix(fmt.Sprintf("http://%v:%v", scrapperHost, scrapperPort), "/tcp")
	}
}

func (s *ServicesHTTPTestSuite) TearDownSuite() {
	s.net.Remove(s.ctx)
	s.botService.Terminate(s.ctx)
	s.scrapperService.Terminate(s.ctx)
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
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, s.scrapperServiceURL()+path, bodyReader)
	s.NoError(err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	s.NoError(err)
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
	s.NoError(err)
	resp, body := s.doRequest(
		http.MethodPost,
		"/links",
		payload,
		map[string]string{"Tg-Chat-Id": strconv.FormatInt(chatID, 10)},
	)
	return resp, body
}

func (s *ServicesHTTPTestSuite) deleteLink(chatID int64, link string) (*http.Response, []byte) {
	payload, err := json.Marshal(map[string]any{
		"link": link,
	})
	s.NoError(err)
	resp, body := s.doRequest(
		http.MethodDelete,
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
	s.NoError(err)
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
	s.NoError(err)
	s.Equal(link, added.URL, "added link url should match")
	s.ElementsMatch(tags, added.Tags, "tags should match")
	s.Greater(added.ID, int64(0), "link id should be set")

	resp, _ = s.addLink(chatID, link, tags)
	s.Equal(http.StatusConflict, resp.StatusCode, "duplicate link should return 409")

	resp, body = s.getLinks(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links is OK for registered chat")
	var listed listLinksResponse
	err = json.Unmarshal(body, &listed)
	s.NoError(err)
	s.Require().Equal(int32(1), listed.Size, "expected one tracked link")
	s.Require().Len(listed.Links, 1, "expected one tracked link in response")
	s.Equal(link, listed.Links[0].URL, "listed link url should match")

	resp, body = s.deleteLink(chatID, link)
	s.Equal(http.StatusOK, resp.StatusCode, "link should be deleted successfully")
	var deleted linkResponse
	err = json.Unmarshal(body, &deleted)
	s.NoError(err)
	s.Equal(link, deleted.URL, "deleted link url should match")

	resp, body = s.getLinks(chatID)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links is OK after delete")
	err = json.Unmarshal(body, &listed)
	s.NoError(err)
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
	s.NoError(err)
	resp, _ := s.doRequest(http.MethodDelete, "/links", payload, nil)
	s.Equal(http.StatusBadRequest, resp.StatusCode, "missing Tg-Chat-Id header: expected 400")
}

func TestSuiteStart(t *testing.T) {
	suite.Run(t, &ServicesHTTPTestSuite{})
}
