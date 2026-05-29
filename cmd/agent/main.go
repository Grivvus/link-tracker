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
	"link-tracker/internal/application/agent/usecase"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/infrastructure"
	"link-tracker/internal/infrastructure/agent"
	"link-tracker/internal/infrastructure/agent/adapters"
	agentKafka "link-tracker/internal/infrastructure/agent/kafka"

	"link-tracker/internal/infrastructure/avro"
	commonKafka "link-tracker/internal/infrastructure/kafka"
)

const agentSettingsFname = "agent-settings.toml"
const groupID = "agent-service-reader"
const shutdownTimeout = 5 * time.Second

type agentRuntime struct {
	logger                *slog.Logger
	rawEventsReader       *kafka.Reader
	processedEventsWriter *kafka.Writer
	dlqWriter             *kafka.Writer
	eventsReader          ports.ReceivedEventsReader
	eventsProcessor       usecase.EventUpdateProcessorUseCase
	grouper               usecase.UpdateGrouper
}

func (app *agentRuntime) run(ctx context.Context) error {
	events, err := app.eventsReader.Start(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := app.eventsProcessor.Process(ctx, event); err != nil {
				app.logger.Error("can't process received event", "err", err)
				if markErr := event.MarkFailed(ctx, err); markErr != nil {
					return markErr
				}
			}
		}
	}
}

func (app *agentRuntime) Close(ctx context.Context) error {
	var err error
	if app.grouper != nil {
		err = errors.Join(err, app.grouper.Flush(ctx))
	}
	if app.dlqWriter != nil {
		err = errors.Join(err, app.dlqWriter.Close())
	}
	if app.rawEventsReader != nil {
		err = errors.Join(err, app.rawEventsReader.Close())
	}
	if app.processedEventsWriter != nil {
		err = errors.Join(err, app.processedEventsWriter.Close())
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

	app, err := newAgentRuntime(ctx, config, rcfg, logger)
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
	ctx context.Context,
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
	summarizer := adapters.NewDummySummarizer(logger, agentSettings.Summarization.Threshold)
	grouper := usecase.NewInMemoryUpdateGrouper(
		ctx, logger,
		time.Duration(agentSettings.Grouping.WindowMS)*time.Millisecond,
		usecase.NewDefaultUpdateMerger(),
		summarizer,
		publisher,
	)

	eventsProcessor := usecase.NewEventUpdateProcessorUseCase(
		usecase.NewUpdateFilter(agentSettings),
		usecase.NewPrioritizer(agentSettings),
		grouper,
	)

	offsetTracker := commonKafka.NewPartitionOffsetTracker(
		logger,
		rawEventsReader,
		dlqWriter,
		codec,
	)
	deferredReader := commonKafka.NewDeferredEventsReader(
		logger,
		rawEventsReader,
		dlqWriter,
		codec,
		offsetTracker,
		rcfg.RetryPolicy,
	)
	agentKafkaReader := agentKafka.NewDeferredRawEventsReader(deferredReader)
	app.processedEventsWriter = processedEventsWriter
	app.dlqWriter = dlqWriter
	app.rawEventsReader = rawEventsReader
	app.eventsReader = agentKafkaReader
	app.eventsProcessor = eventsProcessor
	app.grouper = grouper
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
