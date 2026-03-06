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

type Config struct {
	Token       string `envconfig:"TOKEN" required:"true"`
	BotURL      string `envconfig:"BOT_URL" required:"true"`
	ScrapperURL string `envconfig:"SCRAPPER_URL" required:"true"`
}

func LoadConfig() (Config, error) {
	var cfg Config
	err := godotenv.Load(".env")
	if err != nil {
		return cfg, ErrCantLoadEnvFile
	}
	err = envconfig.Process("", &cfg)
	if err != nil {
		return cfg, ErrCantParseConfig
	}
	return cfg, nil
}
