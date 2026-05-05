package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

var (
	ErrCantLoadEnvFile = errors.New("can't load .env file, possible missing")
	ErrCantParseConfig = errors.New("can't process config")
	ErrInvalidEnv      = errors.New("some environment vairable has invalid value")
)

type Config struct {
	BotToken                  string `envconfig:"BOT_TOKEN" required:"true"`
	BotURL                    string `envconfig:"BOT_URL" required:"true"`
	ServerURL                 string `envconfig:"SERVER_URL" required:"true"`
	ScrapperURL               string `envconfig:"SCRAPPER_URL" required:"true"`
	BotHost                   string `envconfig:"BOT_HOST" required:"true"`
	BotGRPCPort               int    `envconfig:"BOT_GRPC_PORT" required:"true"`
	ScrapperHost              string `envconfig:"SCRAPPER_HOST" required:"true"`
	ScrapperGRPCPort          int    `envconfig:"SCRAPPER_GRPC_PORT" required:"true"`
	UseGRPC                   bool   `envconfig:"USE_GRPC" default:"false"`
	PollerIntervalMinutes     int    `envconfig:"POLLER_INTERVAL_MINUTES" default:"30"`
	DBConnString              string `envconfig:"DB_CONN_STRING" required:"true"`
	DBAccessType              string `envconfig:"DB_ACCESS_TYPE" default:"SQL"`
	ConcurrencyBoundary       uint   `envconfig:"CONCURRENCY_BOUNDARY" default:"1"`
	DBLinksPageSize           int    `envconfig:"DB_LINKS_PAGE_SIZE" default:"50"`
	NotificationMethod        string `envconfig:"NOTIFICATION_METHOD" default:"kafka"`
	KafkaBrokersRaw           string `envconfig:"KAFKA_BROKERS"`
	KafkaTopic                string `envconfig:"KAFKA_TOPIC"`
	KafkaDLQTopic             string `envconfig:"KAFKA_DLQ_TOPIC"`
	KafkaMaxAttempts          int    `envconfig:"KAFKA_MAX_ATTEMPTS" default:"5"`
	SchemaRegistryURL         string `envconfig:"SCHEMA_REGISTRY_URL"`
	ValkeyClusterAddressesRaw string `envconfig:"VALKEY_CLUSTER_ADDRESSES" required:"true"`
	ValkeyPassword            string `envconfig:"VALKEY_PASSWORD" required:"true"`
	ValkeyTTLSeconds          int    `envconfig:"VALKEY_TTL_SECONDS" default:"30"`
}

func (c Config) KafkaBrokers() ([]string, error) {
	if c.NotificationMethod != "kafka" {
		return nil, fmt.Errorf("%w: the http method is selected", ErrInvalidEnv)
	}
	return strings.Split(c.KafkaBrokersRaw, ","), nil
}

func (c Config) ValkeyClusterAddresse() []string {
	return strings.Split(c.ValkeyClusterAddressesRaw, ",")
}

func LoadConfig() (Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return cfg, fmt.Errorf("%w: info - %s", ErrCantParseConfig, err.Error())
	}
	if cfg.DBAccessType != "SQL" && cfg.DBAccessType != "ORM" && cfg.DBAccessType != "MEM" {
		return cfg, fmt.Errorf("%w: invalid DB_ACCESS_TYPE, expect SQL or ORM", err)
	}
	if cfg.ConcurrencyBoundary == 0 {
		return cfg, fmt.Errorf(
			"%w: invalid CONCURRENCY_BOUNDARY, minimum value is 1",
			ErrInvalidEnv,
		)
	}
	if cfg.DBLinksPageSize <= 0 {
		return cfg, fmt.Errorf(
			"%w: invalid DB_LINKS_PAGE_SIZE, should be positive", ErrInvalidEnv,
		)
	}

	if cfg.NotificationMethod != "kafka" && cfg.NotificationMethod != "http" {
		return cfg, fmt.Errorf(
			"%w: invalid notification method, expected kafka or http",
			ErrInvalidEnv,
		)
	}

	if cfg.ValkeyTTLSeconds <= 0 {
		return cfg, fmt.Errorf("%w: ttl must be a positive number", ErrInvalidEnv)
	}

	if cfg.NotificationMethod == "kafka" {
		if cfg.KafkaBrokersRaw == "" {
			return cfg, fmt.Errorf(
				"%w: KAFKA_BROKERS must be set",
				ErrInvalidEnv,
			)
		}
		if cfg.KafkaTopic == "" || cfg.KafkaDLQTopic == "" {
			return cfg, fmt.Errorf("%w: all kafka topics must be set", ErrInvalidEnv)
		}
		if cfg.SchemaRegistryURL == "" {
			return cfg, fmt.Errorf("%w: SCHEMA_REGISTRY_URL must be set", ErrInvalidEnv)
		}
	}
	return cfg, nil
}

type ReliabiltyConfig struct {
	ExternalServicesTimeoutSecond int64 `envconfig:"EXTERNAL_SERVICES_TIMEOUT_SECOND" default:"5"`
	RetryPolicy                   int   `envconfig:"RETRY_POLICY" default:"3"`
	RetryPolicyDelayMS            int64 `envconfig:"RETRY_POLICY_DELAY_MS" default:"100"`
	ReateLimiterMaxBurst          int   `envconfig:"RATE_LIMITER_MAX_BURST" default:"40"`
	RateLimiterRequestsPerSecond  int   `envconfig:"RATE_LIMITER_REQUESTS_PER_SECOND" default:"10"`
}

func NewReliabilityConfig() (ReliabiltyConfig, error) {
	var cfg ReliabiltyConfig
	err := envconfig.Process("", &cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.ExternalServicesTimeoutSecond <= 0 {
		return cfg, fmt.Errorf("%w: ExternalServicesTimeout value must be a positive number", ErrInvalidEnv)
	}
	if cfg.RetryPolicy <= 0 {
		return cfg, fmt.Errorf("%w: RetryPolicy value must be a positive number", ErrInvalidEnv)
	}
	if cfg.RateLimiterRequestsPerSecond <= 0 {
		return cfg, fmt.Errorf("%w: RateLimiterRequestsPerSecond value must be a positive number", ErrInvalidEnv)
	}
	if cfg.ReateLimiterMaxBurst <= 0 {
		return cfg, fmt.Errorf("%w: RateLimiterMaxBurst value must be a positive number", ErrInvalidEnv)
	}
	return cfg, nil
}
