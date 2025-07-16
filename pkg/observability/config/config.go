package config

import (
	"errors"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// Logging
	LoggingLevel  string `split_words:"true" default:"info"`
	LoggingFormat string `split_words:"true" default:"json"`

	// Tracing
	TracingName    string `split_words:"true" default:"github.com/renatoaraujo/go-monorepo-playground"`
	TracingEnabled bool   `split_words:"true" required:"true" default:"true"`
	TracingURL     string `split_words:"true" default:"otel-collector:4317"`

	// Sentry
	SentryEnabled              bool          `split_words:"true" required:"false" default:"false"`
	SentryDsn                  string        `split_words:"true" required:"false"`
	SentryDebugEnabled         bool          `split_words:"true" default:"false"`
	SentryTracingEnabled       bool          `split_words:"true" default:"false"`
	SentryTracingSampleRate    float64       `split_words:"true" default:"1.0"`
	SentryFlushTimeoutDuration time.Duration `split_words:"true" default:"2s"`

	// Service details
	ServiceName    string `split_words:"true" required:"true" default:"go-monorepo-playground"`
	Environment    string `split_words:"true" required:"true" default:"development"`
	ReleaseVersion string `split_words:"true" required:"true" default:"development"`

	PyroscopeProfileTypes         []string `split_words:"true" default:"cpu,mem,block,mutex,heap,gc,threadcreate,trace"`
	PyroscopeEnableMutexProfiling bool     `split_words:"true" default:"false"`
	PyroscopeMutexProfileFraction int      `split_words:"true" default:"100"`
	PyroscopeEnableBlockProfiling bool     `split_words:"true" default:"false"`
	PyroscopeBlockProfileRate     int      `split_words:"true" default:"10000"`
	PyroscopeURL                  string   `split_words:"true" default:"http://pyroscope:4040"`
	PyroscopeAuthToken            string   `split_words:"true" default:""`
	PyroscopeEnabled              bool     `split_words:"true" default:"true"`
}

func Init(releaseVersion string) (*Config, error) {
	var cfg Config

	if err := envconfig.Process("OBSERVABILITY", &cfg); err != nil {
		return nil, errors.Join(err, errors.New("failed to load configuration"))
	}

	cfg.ReleaseVersion = releaseVersion

	return &cfg, nil
}
