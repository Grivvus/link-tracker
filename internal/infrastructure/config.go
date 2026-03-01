package infrastructure

import (
	"errors"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

var (
	ErrCantLoadEnvFile = errors.New("can't load .env file, possible missing")
	ErrCantParseConfig = errors.New("can't process config")
)

type BotConfig struct {
	Token string `envconfig:"TOKEN" required:"true"`
	Host  string `envconfig:"HOST" required:"true"`
	Port  uint   `envconfig:"PORT" required:"true"`
}

func LoadBotConfig() (BotConfig, error) {
	var cfg BotConfig
	err := godotenv.Load(".env")
	if err != nil {
		return cfg, ErrCantLoadEnvFile
	}
	err = envconfig.Process("bot", &cfg)
	if err != nil {
		return cfg, ErrCantParseConfig
	}
	return cfg, nil
}
