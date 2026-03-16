package tests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func (s *ServicesHTTPTestSuite) TestListCommand_ListFailsIfNotRegistered() {
	fmt.Println("TestListCommand_ListFailsIfNotRegistered", s.scrapperServiceURL())
	req, err := http.NewRequest(http.MethodGet, s.scrapperServiceURL()+"/links", nil)
	s.NoError(err)
	req.Header.Set("Tg-Chat-Id", "321")
	resp, err := http.DefaultClient.Do(req)
	s.NoError(err)
	s.Equal(http.StatusNotFound, resp.StatusCode, "chat not registered: expected response is 404")
}

func (s *ServicesHTTPTestSuite) TestListCommand_ListAfterChatRegistration() {
	fmt.Println("TestListCommand_ListAfterChatRegistration", s.scrapperServiceURL())
	fmt.Println(s.scrapperServiceURL() + "/tg-chat/123")
	req, err := http.NewRequest(http.MethodPost, s.scrapperServiceURL()+"/tg-chat/123", strings.NewReader(""))
	s.NoError(err)
	resp, err := http.DefaultClient.Do(req)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode, "chat should be registered successfully")

	req, err = http.NewRequest(http.MethodGet, s.scrapperServiceURL()+"/links", nil)
	s.NoError(err)
	req.Header.Set("Tg-Chat-Id", "123")
	resp, err = http.DefaultClient.Do(req)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode, "getting links are fine now")
	body, _ := io.ReadAll(resp.Body)
	s.NotEmpty(body, "body shouldn't be empty")
}

func TestSuiteStart(t *testing.T) {
	suite.Run(t, &ServicesHTTPTestSuite{})
}
