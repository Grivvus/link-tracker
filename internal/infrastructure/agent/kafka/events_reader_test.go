//go:build integrational

package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	kafkaClient "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"link-tracker/internal/application/agent/usecase"
	"link-tracker/internal/application/model"
	"link-tracker/internal/infrastructure/agent"
	"link-tracker/internal/infrastructure/agent/adapters"
	agentKafka "link-tracker/internal/infrastructure/agent/kafka"
	"link-tracker/internal/infrastructure/avro"
	commonKafka "link-tracker/internal/infrastructure/kafka"
)

const agentReaderGroupID = "agent-events-reader"
const agentRawTopicPrefix = "test-agent-raw-topic"
const agentProcessedTopicPrefix = "test-agent-processed-topic"
const agentDLQTopicPrefix = "test-agent-dlq-topic"
const agentClusterID = "MkU3OEVBNTcwNTJENDM2Qk"
const agentBrokerAddr = "127.0.0.1:29093"
const agentBrokerPort = "29093"
const agentAvroSchemaPath = "../../../../avro/link_update.avsc"
const agentTestSchemaID = 77

var agentMsg = model.LinkUpdatedEvent{
	Description:  "description 123",
	LinkID:       123,
	ChatIDs:      []int64{123, 124},
	UpdateName:   "update 123",
	UpdateAuthor: "Ivanovich",
	UpdatedAt:    time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	RawURL:       "https://github.com/torvalds/linux",
}

type noopSummarizer struct{}

func (noopSummarizer) Summarize(
	_ context.Context, event model.LinkUpdatedEvent,
) (model.LinkUpdatedEvent, error) {
	return event, nil
}

type staticMerger struct{}

func (staticMerger) Merge(int64, []model.LinkUpdatedEvent) model.LinkUpdatedEvent {
	return agentMsg
}

type RawEventsReaderIntegrationSuite struct {
	suite.Suite
	ctx            context.Context
	readerGroupID  string
	rawTopic       string
	processedTopic string
	dlqTopic       string
	kafkaService   testcontainers.Container
	avroCodec      *avro.Codec

	rawWriter       *kafkaClient.Writer
	rawReader       *kafkaClient.Reader
	processedWriter *kafkaClient.Writer
	processedReader *kafkaClient.Reader
	dlqWriter       *kafkaClient.Writer
	dlqReader       *kafkaClient.Reader
}

func TestRawEventsReaderIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RawEventsReaderIntegrationSuite))
}

func (s *RawEventsReaderIntegrationSuite) SetupSuite() {
	kafkaContainer := testcontainers.ContainerRequest{
		Image:        "apache/kafka:4.0.2",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                                  "1",
			"KAFKA_PROCESS_ROLES":                            "controller,broker",
			"KAFKA_LISTENERS":                                "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093",
			"KAFKA_ADVERTISED_LISTENERS":                     fmt.Sprintf("PLAINTEXT://%s", agentBrokerAddr),
			"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":                 "1@127.0.0.1:9093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":               "PLAINTEXT",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
			"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":         "0",
			"KAFKA_NUM_PARTITIONS":                           "3",
			"CLUSTER_ID":                                     agentClusterID,
		},
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = nat.PortMap{
				"9092/tcp": []nat.PortBinding{{
					HostIP:   "127.0.0.1",
					HostPort: agentBrokerPort,
				}},
			}
		},
		WaitingFor: wait.ForListeningPort("9092/tcp").
			WithStartupTimeout(2 * time.Minute).
			WithPollInterval(time.Second),
	}

	var err error
	s.kafkaService, err = startKafkaContainer(context.Background(), kafkaContainer)
	if err != nil {
		s.T().Skipf("docker is required for kafka integration tests: %v", err)
		return
	}
	s.Require().NoError(waitBrokerReady(context.Background(), agentBrokerAddr))

	schema, err := os.ReadFile(agentAvroSchemaPath)
	s.Require().NoError(err)
	s.avroCodec, err = avro.NewCodecFromSchema(string(schema), agentTestSchemaID)
	s.Require().NoError(err)
}

func startKafkaContainer(
	ctx context.Context,
	req testcontainers.ContainerRequest,
) (container testcontainers.Container, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("start kafka container: %v", recovered)
		}
	}()

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func (s *RawEventsReaderIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	testSuffix := normalizeKafkaNameSuffix(s.T().Name())
	s.readerGroupID = fmt.Sprintf("%s-%s", agentReaderGroupID, testSuffix)
	s.rawTopic = fmt.Sprintf("%s-%s", agentRawTopicPrefix, testSuffix)
	s.processedTopic = fmt.Sprintf("%s-%s", agentProcessedTopicPrefix, testSuffix)
	s.dlqTopic = fmt.Sprintf("%s-%s", agentDLQTopicPrefix, testSuffix)

	s.Require().NoError(createKafkaTopics(s.ctx, agentBrokerAddr, s.rawTopic, s.processedTopic, s.dlqTopic))

	s.rawWriter = &kafkaClient.Writer{
		Addr:  kafkaClient.TCP(agentBrokerAddr),
		Topic: s.rawTopic,
	}
	s.rawReader = kafkaClient.NewReader(kafkaClient.ReaderConfig{
		Brokers: []string{agentBrokerAddr},
		GroupID: s.readerGroupID,
		Topic:   s.rawTopic,
	})
	s.processedWriter = &kafkaClient.Writer{
		Addr:  kafkaClient.TCP(agentBrokerAddr),
		Topic: s.processedTopic,
	}
	s.processedReader = kafkaClient.NewReader(kafkaClient.ReaderConfig{
		Brokers:   []string{agentBrokerAddr},
		Topic:     s.processedTopic,
		Partition: 0,
	})
	_ = s.processedReader.SetOffset(kafkaClient.FirstOffset)
	s.dlqWriter = &kafkaClient.Writer{
		Addr:  kafkaClient.TCP(agentBrokerAddr),
		Topic: s.dlqTopic,
	}
	s.dlqReader = kafkaClient.NewReader(kafkaClient.ReaderConfig{
		Brokers:   []string{agentBrokerAddr},
		Topic:     s.dlqTopic,
		Partition: 0,
	})
	_ = s.dlqReader.SetOffset(kafkaClient.FirstOffset)
}

func (s *RawEventsReaderIntegrationSuite) TearDownTest() {
	if s.rawReader != nil {
		_ = s.rawReader.Close()
	}
	if s.rawWriter != nil {
		_ = s.rawWriter.Close()
	}
	if s.processedReader != nil {
		_ = s.processedReader.Close()
	}
	if s.processedWriter != nil {
		_ = s.processedWriter.Close()
	}
	if s.dlqReader != nil {
		_ = s.dlqReader.Close()
	}
	if s.dlqWriter != nil {
		_ = s.dlqWriter.Close()
	}
}

func (s *RawEventsReaderIntegrationSuite) TearDownSuite() {
	if s.kafkaService != nil {
		_ = s.kafkaService.Terminate(context.Background())
	}
}

func (s *RawEventsReaderIntegrationSuite) TestRawEventsReader_ProcessesValidRawMessage() {
	msgKey, err := json.Marshal(agentMsg.LinkID)
	s.Require().NoError(err)
	msgValue, err := s.avroCodec.Encode(agentMsg)
	s.Require().NoError(err)

	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()

	eventsReader := s.newRawEventsReader()
	eventsReader.Start(ctx)
	s.waitReaderReady(s.rawReader)
	s.writeRawMessageAndWaitReady(ctx, kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})

	processedEvent := s.readProcessedEvent()
	s.Require().Equal(agentMsg, processedEvent)
	s.assertDLQIsEmpty()
}

func (s *RawEventsReaderIntegrationSuite) TestRawEventsReader_WritesInvalidMessageToDLQAndContinues() {
	msgKey, err := json.Marshal(agentMsg.LinkID)
	s.Require().NoError(err)

	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()

	eventsReader := s.newRawEventsReader()
	eventsReader.Start(ctx)
	s.waitReaderReady(s.rawReader)

	invalidMessage := kafkaClient.Message{
		Key:   msgKey,
		Value: []byte("bad"),
	}
	s.writeRawMessageAndWaitReady(ctx, invalidMessage)
	s.assertSingleMessageInDLQ(invalidMessage)

	msgValue, err := s.avroCodec.Encode(agentMsg)
	s.Require().NoError(err)
	s.writeRawMessageAndWaitReady(ctx, kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})

	processedEvent := s.readProcessedEvent()
	s.Require().Equal(agentMsg, processedEvent)
}

func (s *RawEventsReaderIntegrationSuite) newRawEventsReader() *agentKafka.RawEventsReader {
	settings := agent.AgentSettings{
		Filtering: agent.Filtering{
			MinLength: 1,
		},
		Summarization: agent.Summarization{
			Threshold: 1000,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := adapters.NewProcessedUpdatePublisher(logger, s.processedWriter, s.avroCodec)
	grouper := usecase.NewInMemoryUpdateGrouper(
		context.Background(),
		logger,
		time.Nanosecond,
		staticMerger{},
		noopSummarizer{},
		publisher,
	)
	processor := usecase.NewEventUpdateProcessorUseCase(
		usecase.NewUpdateFilter(settings),
		usecase.NewPrioritizer(settings),
		grouper,
	)
	return agentKafka.NewRawEventsReader(*commonKafka.NewEventsReader(
		logger,
		commonKafka.HandlerFromEventsProcessor(processor),
		s.rawReader,
		s.dlqWriter,
		s.avroCodec,
		3,
	))
}

func (s *RawEventsReaderIntegrationSuite) writeRawMessageAndWaitReady(
	ctx context.Context, msg kafkaClient.Message,
) {
	var err error
	s.Require().Eventually(func() bool {
		err = s.rawWriter.WriteMessages(ctx, msg)
		return err == nil || !isRetriableKafkaWriteError(err)
	}, 10*time.Second, 100*time.Millisecond)
	s.Require().NoError(err)
}

func (s *RawEventsReaderIntegrationSuite) readProcessedEvent() model.LinkUpdatedEvent {
	readCtx, readCancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer readCancel()

	processedMsg, err := s.processedReader.FetchMessage(readCtx)
	s.Require().NoError(err)
	processedEvent, err := s.avroCodec.Decode(processedMsg.Value)
	s.Require().NoError(err)
	return processedEvent
}

func (s *RawEventsReaderIntegrationSuite) assertSingleMessageInDLQ(expectedMsg kafkaClient.Message) {
	readCtx, readCancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer readCancel()

	dlqMsg, err := s.dlqReader.FetchMessage(readCtx)
	s.Require().NoError(err)
	s.Require().Equal(expectedMsg.Key, dlqMsg.Key)
	s.Require().Equal(expectedMsg.Value, dlqMsg.Value)
	s.assertDLQIsEmpty()
}

func (s *RawEventsReaderIntegrationSuite) assertDLQIsEmpty() {
	noMoreCtx, noMoreCancel := context.WithTimeout(s.ctx, time.Second)
	defer noMoreCancel()

	_, err := s.dlqReader.FetchMessage(noMoreCtx)
	s.Require().Error(err)
	s.Require().ErrorIs(err, context.DeadlineExceeded)
}

func (s *RawEventsReaderIntegrationSuite) waitReaderReady(reader *kafkaClient.Reader) {
	s.Require().Eventually(func() bool {
		return reader.Stats().Rebalances > 0
	}, 10*time.Second, 100*time.Millisecond)
}

func createKafkaTopics(ctx context.Context, broker string, topics ...string) error {
	conn, err := kafkaClient.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafkaClient.DialContext(
		ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)),
	)
	if err != nil {
		return err
	}
	defer func() { _ = controllerConn.Close() }()

	topicConfigs := make([]kafkaClient.TopicConfig, 0, len(topics))
	for _, topicName := range topics {
		topicConfigs = append(topicConfigs, kafkaClient.TopicConfig{
			Topic:             topicName,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}

	if err := controllerConn.CreateTopics(topicConfigs...); err != nil {
		return err
	}

	for _, topicName := range topics {
		if err := waitTopicReady(ctx, broker, topicName); err != nil {
			return err
		}
	}

	return nil
}

func isRetriableKafkaWriteError(err error) bool {
	return errors.Is(err, kafkaClient.UnknownTopicOrPartition) ||
		errors.Is(err, kafkaClient.LeaderNotAvailable)
}

func waitTopicReady(ctx context.Context, broker, topic string) error {
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for {
		conn, err := kafkaClient.DialLeader(waitCtx, "tcp", broker, topic, 0)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if waitCtx.Err() != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitBrokerReady(ctx context.Context, broker string) error {
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		conn, err := kafkaClient.DialContext(waitCtx, "tcp", broker)
		if err == nil {
			_, err = conn.Controller()
			_ = conn.Close()
			if err == nil {
				return nil
			}
		}
		if waitCtx.Err() != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func normalizeKafkaNameSuffix(name string) string {
	var suffix strings.Builder
	suffix.Grow(len(name))

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			suffix.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			suffix.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			suffix.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			suffix.WriteRune(r)
		default:
			suffix.WriteByte('-')
		}
	}

	return strings.Trim(suffix.String(), "-")
}
