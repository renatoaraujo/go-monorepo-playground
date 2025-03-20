package config

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port        string `default:"8080"`
	Environment string `required:"true" default:"development" split_words:"true"`
	ServiceName string `split_words:"true" required:"true"`
}

func Init(ctx context.Context) (*Config, error) {
	var cfg Config

	env := os.Getenv("ENVIRONMENT")
	if env == "local" || env == "development" {
		if err := godotenv.Load(); err != nil {
			slog.InfoContext(ctx, "no .env file found, using environment variables")
		}
	}

	if err := envconfig.Process("", &cfg); err != nil {
		slog.InfoContext(ctx, "failed to load configuration", "error", err)
		return nil, err
	}

	return &cfg, nil
}
