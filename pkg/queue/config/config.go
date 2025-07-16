package config

import (
	"errors"
	"github.com/kelseyhightower/envconfig"
	"time"
)

type Config struct {
	// Logging
	LoggingLevel  string `split_words:"true" default:"info"`
	LoggingFormat string `split_words:"true" default:"json"`

	// Service details
	ServiceName    string `split_words:"true" required:"true" default:"go-monorepo-playground"`
	Environment    string `split_words:"true" required:"true" default:"development"`
	ReleaseVersion string `split_words:"true" required:"true" default:"development"`

	NatsEnabled       bool          `split_words:"true" required:"true" default:"true"`
	NatsURL           string        `split_words:"true" required:"true" default:"nats:4222"`
	NatsClientName    string        `split_words:"true" required:"true" default:"go-monorepo-playground"`
	NatsReconnectWait time.Duration `split_words:"true" default:"2s"`
	NatsMaxReconnects int           `split_words:"true" default:"-1"`
}

func Init(releaseVersion string) (*Config, error) {
	var cfg Config

	if err := envconfig.Process("QUEUE", &cfg); err != nil {
		return nil, errors.Join(err, errors.New("failed to load configuration"))
	}

	cfg.ReleaseVersion = releaseVersion

	return &cfg, nil
}
