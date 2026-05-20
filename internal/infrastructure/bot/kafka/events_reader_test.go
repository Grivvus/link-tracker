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
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	kafkaClient "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot/kafka"
	commonKafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
)

const groupID = "bot-events-reader"
const topicPrefix = "test-topic"
const dlqTopicPrefix = "test-dlq-topic"
const clusterID = "MkU3OEVBNTcwNTJENDM2Qk"
const brokerAddr = "127.0.0.1:29092"
const brokerPort = "29092"
const avroSchemaPath = "../../../../avro/link_update.avsc"
const testSchemaID = 66

type BotAdapterTestImpl struct {
	mu           sync.RWMutex
	SentMessages []model.LinkUpdatedEvent
}

func (bot *BotAdapterTestImpl) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	bot.SentMessages = append(bot.SentMessages, event)
	return nil
}

type BotAdapterFailingImpl struct {
	SentMessages []model.LinkUpdatedEvent
}

func (bot *BotAdapterFailingImpl) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	return fmt.Errorf("can't write message")
}

type BotAdapterFailingExact3Times struct {
	mu           sync.RWMutex
	Cnt          int
	SentMessages []model.LinkUpdatedEvent
}

func (bot *BotAdapterFailingExact3Times) NotifyAboutUpdatedLinks(
	ctx context.Context, event model.LinkUpdatedEvent,
) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	if bot.Cnt == 3 {
		bot.Cnt = 0
		bot.SentMessages = append(bot.SentMessages, event)
		return nil
	}
	bot.Cnt++
	return fmt.Errorf("can't write message")
}

const msgID = 123

var msg = model.LinkUpdatedEvent{
	Description:  "description 123",
	LinkID:       msgID,
	ChatIDs:      []int64{123, 124},
	UpdateName:   "update 123",
	UpdateAuthor: "Ivanovich",
	UpdatedAt:    time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	RawURL:       "https://github.com/torvalds/linux",
}

type EventsReaderTestSuite struct {
	suite.Suite
	ctx           context.Context
	readerGroupID string
	eventsTopic   string
	dlqTopic      string
	kafkaService  testcontainers.Container
	avroCodec     *avro.Codec

	kafkaEventsWriter *kafkaClient.Writer
	kafkaEventsReader *kafkaClient.Reader
	kafkaDLQWriter    *kafkaClient.Writer
	kafkaDLQReader    *kafkaClient.Reader
}

func (s *EventsReaderTestSuite) SetupSuite() {
	kafkaContainer := testcontainers.ContainerRequest{
		Image:        "apache/kafka:4.0.2",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                                  "1",
			"KAFKA_PROCESS_ROLES":                            "controller,broker",
			"KAFKA_LISTENERS":                                "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093",
			"KAFKA_ADVERTISED_LISTENERS":                     fmt.Sprintf("PLAINTEXT://%s", brokerAddr),
			"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":                 "1@127.0.0.1:9093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":               "PLAINTEXT",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
			"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":         "0",
			"KAFKA_NUM_PARTITIONS":                           "3",
			"CLUSTER_ID":                                     clusterID,
		},
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = nat.PortMap{
				"9092/tcp": []nat.PortBinding{{
					HostIP:   "127.0.0.1",
					HostPort: brokerPort,
				}},
			}
		},
		WaitingFor: wait.ForListeningPort("9092/tcp").
			WithStartupTimeout(2 * time.Minute).
			WithPollInterval(time.Second),
	}

	var err error
	s.kafkaService, err = testcontainers.GenericContainer(
		context.Background(), testcontainers.GenericContainerRequest{
			ContainerRequest: kafkaContainer,
			Started:          true,
		})
	s.Require().NoError(err)
	s.Require().NoError(waitBrokerReady(context.Background(), brokerAddr))

	schema, err := os.ReadFile(avroSchemaPath)
	s.Require().NoError(err)
	s.avroCodec, err = avro.NewCodecFromSchema(string(schema), testSchemaID)
	s.Require().NoError(err)
}

func (s *EventsReaderTestSuite) SetupTest() {
	s.ctx = context.Background()
	testSuffix := normalizeKafkaNameSuffix(s.T().Name())
	s.readerGroupID = fmt.Sprintf("%s-%s", groupID, testSuffix)
	s.eventsTopic = fmt.Sprintf("%s-%s", topicPrefix, testSuffix)
	s.dlqTopic = fmt.Sprintf("%s-%s", dlqTopicPrefix, testSuffix)

	s.Require().NoError(createKafkaTopics(s.ctx, brokerAddr, s.eventsTopic, s.dlqTopic))
	s.kafkaDLQWriter = &kafkaClient.Writer{
		Addr:  kafkaClient.TCP(brokerAddr),
		Topic: s.dlqTopic,
	}
	s.kafkaEventsReader = kafkaClient.NewReader(kafkaClient.ReaderConfig{
		Brokers: []string{brokerAddr},
		GroupID: s.readerGroupID,
		Topic:   s.eventsTopic,
	})
	s.kafkaDLQReader = kafkaClient.NewReader(kafkaClient.ReaderConfig{
		Brokers:   []string{brokerAddr},
		Topic:     s.dlqTopic,
		Partition: 0,
	})
	_ = s.kafkaDLQReader.SetOffset(kafkaClient.FirstOffset)
	s.kafkaEventsWriter = &kafkaClient.Writer{
		Addr:  kafkaClient.TCP(brokerAddr),
		Topic: s.eventsTopic,
	}
}

func (s *EventsReaderTestSuite) TearDownTest() {
	if s.kafkaEventsReader != nil {
		_ = s.kafkaEventsReader.Close()
	}
	if s.kafkaDLQReader != nil {
		_ = s.kafkaDLQReader.Close()
	}
	if s.kafkaEventsWriter != nil {
		_ = s.kafkaEventsWriter.Close()
	}
	if s.kafkaDLQWriter != nil {
		_ = s.kafkaDLQWriter.Close()
	}
}

func (s *EventsReaderTestSuite) TearDownSuite() {
	if s.kafkaService != nil {
		_ = s.kafkaService.Terminate(context.Background())
	}
}

func TestEventsReaderTestSuite(t *testing.T) {
	suite.Run(t, new(EventsReaderTestSuite))
}

func (s *EventsReaderTestSuite) TestEventsReader_ReadNewValidMessage() {
	msgKey, err := json.Marshal(msgID)
	s.Require().NoError(err)
	msgValue, err := s.avroCodec.Encode(msg)
	s.Require().NoError(err)

	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()
	botAdapter := &BotAdapterTestImpl{}
	eventsReader := s.newEventsReader(botAdapter, 5)
	eventsReader.Start(ctx)
	s.waitReaderReady(s.kafkaEventsReader)
	s.writeMessageAndWaitReady(ctx, kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})
	s.Assert().Eventually(func() bool {
		botAdapter.mu.RLock()
		defer botAdapter.mu.RUnlock()
		return len(botAdapter.SentMessages) == 1
	}, 10*time.Second, 200*time.Millisecond)
	botAdapter.mu.RLock()
	defer botAdapter.mu.RUnlock()
	s.Require().Len(botAdapter.SentMessages, 1)
	s.Assert().Equal(msg, botAdapter.SentMessages[0])
}

func (s *EventsReaderTestSuite) TestEventsReader_WriteToDLQAfterDeserializationError() {
	msgKey, err := json.Marshal(msgID)
	s.Require().NoError(err)

	botAdapter := &BotAdapterTestImpl{}
	s.writeInvalidMessageAndAssertDLQ(botAdapter, kafkaClient.Message{
		Key:   msgKey,
		Value: []byte("bad"),
	})

	botAdapter.mu.RLock()
	defer botAdapter.mu.RUnlock()
	s.Assert().Empty(botAdapter.SentMessages)
}

func (s *EventsReaderTestSuite) TestEventsReader_DoesNotWriteToDLQWhenRetryPolicyNotExceeded() {
	msgKey, err := json.Marshal(msgID)
	s.Require().NoError(err)
	msgValue, err := s.avroCodec.Encode(msg)
	s.Require().NoError(err)

	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()

	botAdapter := &BotAdapterFailingExact3Times{}
	eventsReader := s.newEventsReader(botAdapter, 5)
	eventsReader.Start(ctx)
	s.waitReaderReady(s.kafkaEventsReader)
	s.writeMessageAndWaitReady(ctx, kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})

	s.Assert().Eventually(func() bool {
		botAdapter.mu.RLock()
		defer botAdapter.mu.RUnlock()
		return len(botAdapter.SentMessages) == 1
	}, 10*time.Second, 200*time.Millisecond)
	botAdapter.mu.RLock()
	defer botAdapter.mu.RUnlock()
	s.Require().Len(botAdapter.SentMessages, 1)
	s.Assert().Equal(msg, botAdapter.SentMessages[0])
	s.assertDLQIsEmpty()
}

func (s *EventsReaderTestSuite) TestEventsReader_WritesToDLQWhenRetryPolicyExceeded() {
	msgKey, err := json.Marshal(msgID)
	s.Require().NoError(err)
	msgValue, err := s.avroCodec.Encode(msg)
	s.Require().NoError(err)

	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()

	botAdapter := &BotAdapterFailingImpl{}
	eventsReader := s.newEventsReader(botAdapter, 3)
	eventsReader.Start(ctx)
	s.waitReaderReady(s.kafkaEventsReader)
	s.writeMessageAndWaitReady(ctx, kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})

	s.Assert().Empty(botAdapter.SentMessages)
	s.assertSingleMessageInDLQ(kafkaClient.Message{
		Key:   msgKey,
		Value: msgValue,
	})
}

func (s *EventsReaderTestSuite) writeInvalidMessageAndAssertDLQ(
	botAdapter *BotAdapterTestImpl, msg kafkaClient.Message,
) {
	ctx, cancelFunc := context.WithCancel(s.ctx)
	defer cancelFunc()

	eventsReader := s.newEventsReader(botAdapter, 5)
	eventsReader.Start(ctx)
	s.waitReaderReady(s.kafkaEventsReader)

	s.writeMessageAndWaitReady(ctx, msg)

	readCtx, readCancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer readCancel()

	dlqMsg, err := s.kafkaDLQReader.FetchMessage(readCtx)
	s.Require().NoError(err)
	s.Assert().Equal(msg.Key, dlqMsg.Key)
	s.Assert().Equal(msg.Value, dlqMsg.Value)

	s.assertDLQIsEmpty()
}

func (s *EventsReaderTestSuite) newEventsReader(
	botAdapter ports.BotPort, retryPolicy int,
) *kafka.EventsReader {
	return kafka.NewEventsReader(*commonKafka.NewEventsReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		commonKafka.HandlerFromBotPort(botAdapter),
		s.kafkaEventsReader,
		s.kafkaDLQWriter,
		s.avroCodec,
		retryPolicy,
	))
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

func (s *EventsReaderTestSuite) waitReaderReady(reader *kafkaClient.Reader) {
	s.Require().Eventually(func() bool {
		return reader.Stats().Rebalances > 0
	}, 10*time.Second, 100*time.Millisecond)
}

func (s *EventsReaderTestSuite) writeMessageAndWaitReady(
	ctx context.Context, msg kafkaClient.Message,
) {
	var err error
	s.Require().Eventually(func() bool {
		err = s.kafkaEventsWriter.WriteMessages(ctx, msg)
		return err == nil || !isRetriableKafkaWriteError(err)
	}, 10*time.Second, 100*time.Millisecond)
	s.Require().NoError(err)
}

func (s *EventsReaderTestSuite) assertSingleMessageInDLQ(expectedMsg kafkaClient.Message) {
	readCtx, readCancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer readCancel()

	dlqMsg, err := s.kafkaDLQReader.FetchMessage(readCtx)
	s.Require().NoError(err)
	s.Assert().Equal(expectedMsg.Key, dlqMsg.Key)
	s.Assert().Equal(expectedMsg.Value, dlqMsg.Value)
	s.assertDLQIsEmpty()
}

func (s *EventsReaderTestSuite) assertDLQIsEmpty() {
	noMoreCtx, noMoreCancel := context.WithTimeout(s.ctx, time.Second)
	defer noMoreCancel()

	_, err := s.kafkaDLQReader.FetchMessage(noMoreCtx)
	s.Require().Error(err)
	s.ErrorIs(err, context.DeadlineExceeded)
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
