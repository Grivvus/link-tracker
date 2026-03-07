package infrastructure

import (
	"errors"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

var (
	ErrCantLoadEnvFile = errors.New("can't load .env file, possible missing")
	ErrCantParseConfig = errors.New("can't process config")
)

type Config struct {
	BotToken    string `envconfig:"BOT_TOKEN" required:"true"`
	BotURL      string `envconfig:"BOT_URL" required:"true"`
	ScrapperURL string `envconfig:"SCRAPPER_URL" required:"true"`
}

func LoadConfig() (Config, error) {
	var cfg Config
	err := godotenv.Load(".env")
	if err != nil {
		return cfg, fmt.Errorf("%w: info - %s", ErrCantLoadEnvFile, err.Error())
	}
	err = envconfig.Process("", &cfg)
	if err != nil {
		return cfg, fmt.Errorf("%w: info - %s", ErrCantParseConfig, err.Error())
	}
	return cfg, nil
}
