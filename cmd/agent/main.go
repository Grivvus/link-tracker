package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent/usecase"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent/adapters"
	agentKafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/agent/kafka"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/avro"
	commonKafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
)

const agentSettingsFname = "agent-settings.toml"
const groupID = "agent-service-reader"
const shutdownTimeout = 5 * time.Second

type agentRuntime struct {
	logger                *slog.Logger
	rawEventsReader       *kafka.Reader
	processedEventsWriter *kafka.Writer
	dlqWriter             *kafka.Writer
	eventsReader          *agentKafka.RawEventsReader
}

func (app *agentRuntime) run(ctx context.Context) error {
	app.eventsReader.Start(ctx)
	return nil
}

func (app *agentRuntime) Close(ctx context.Context) error {
	var err error
	if app.dlqWriter != nil {
		errors.Join(err, app.dlqWriter.Close())
	}
	if app.rawEventsReader != nil {
		errors.Join(err, app.rawEventsReader.Close())
	}
	if app.processedEventsWriter != nil {
		errors.Join(err, app.processedEventsWriter.Close())
	}
	return err
}

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}),
	)

	if err := run(logger); err != nil {
		logger.Error("Agent service exit with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	config, err := infrastructure.LoadConfig()
	if err != nil {
		return err
	}
	rcfg, err := infrastructure.NewReliabilityConfig()
	if err != nil {
		return err
	}

	app, err := newAgentRuntime(config, rcfg, logger)
	if err != nil {
		return err
	}

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		err = errors.Join(err, app.Close(shutdownCtx))
	}()

	return app.run(ctx)
}

func newAgentRuntime(
	cfg infrastructure.Config,
	rcfg infrastructure.ReliabiltyConfig, logger *slog.Logger,
) (app agentRuntime, err error) {
	agentSettings, err := agent.NewAgentSettings(agentSettingsFname)
	if err != nil {
		return agentRuntime{}, err
	}

	rawEventsReader, processedEventsWriter, dlqWriter, codec, err := initKafka(&cfg, logger)
	if err != nil {
		return agentRuntime{}, err
	}
	publisher := adapters.NewProcessedUpdatePublisher(logger, processedEventsWriter, codec)
	summarizer := adapters.NewDummySummarizer(logger)

	eventsProcessor := usecase.NewEventUpdateProcessorUseCase(
		usecase.NewUpdateFilter(agentSettings),
		summarizer,
		publisher,
		agentSettings.Summarization.Threshold,
	)

	agentKafkaReader := agentKafka.NewRawEventsReader(*commonKafka.NewEventsReader(
		logger, commonKafka.HandlerFromEventsProcessor(eventsProcessor),
		rawEventsReader, dlqWriter, codec, rcfg.RetryPolicy))
	app.processedEventsWriter = processedEventsWriter
	app.dlqWriter = dlqWriter
	app.rawEventsReader = rawEventsReader
	app.eventsReader = agentKafkaReader
	return app, err

}

func initKafka(
	cfg *infrastructure.Config,
	logger *slog.Logger,
) (rawEventsReader *kafka.Reader, processedEventsWriter *kafka.Writer, dlqWriter *kafka.Writer, codec *avro.Codec, err error) {
	brokers, err := cfg.KafkaBrokers()
	if err != nil {
		logger.Error("invalid kafka brokers", "err", err)
		return nil, nil, nil, nil, err
	}
	processedWriter := &kafka.Writer{
		Addr:        kafka.TCP(brokers...),
		Topic:       cfg.KafkaProcessedTopic,
		Balancer:    &kafka.LeastBytes{},
		Logger:      kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger: kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
	}
	rawNotificationsReader := kafka.NewReader(kafka.ReaderConfig{
		Topic:       cfg.KafkaTopic,
		Brokers:     brokers,
		Logger:      kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger: kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
		GroupID:     groupID,
	})

	dlqWriter = &kafka.Writer{
		Addr:        kafka.TCP(brokers...),
		Topic:       cfg.KafkaDLQTopic,
		Balancer:    &kafka.LeastBytes{},
		Logger:      kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Info)),
		ErrorLogger: kafka.LoggerFunc(infrastructure.KafkaLoggerFn(logger.Error)),
	}
	codec, err = avro.NewCodec(
		cfg.SchemaRegistryURL, cfg.KafkaTopic, "/link_update.avsc",
	)
	if err != nil {
		logger.Error("can't create avro codec", "err", err)
		return nil, nil, nil, nil, err
	}
	return rawNotificationsReader, processedWriter, dlqWriter, codec, nil
}
