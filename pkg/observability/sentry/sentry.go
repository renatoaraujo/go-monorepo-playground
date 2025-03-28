package sentry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"

	"github.com/getsentry/sentry-go"
	sentryslog "github.com/getsentry/sentry-go/slog"
	slogotel "github.com/samber/slog-otel"
)

type Client struct {
	flushDuration time.Duration
}

func Init(cfg *config.Config) (*Client, error) {
	err := sentry.Init(sentry.ClientOptions{
		Release:          cfg.ReleaseVersion,
		Dsn:              cfg.SentryDsn,
		Debug:            cfg.SentryDebugEnabled,
		EnableTracing:    cfg.SentryTracingEnabled,
		TracesSampleRate: cfg.SentryTracingSampleRate,
		Environment:      cfg.Environment,
	})

	return &Client{flushDuration: cfg.SentryFlushTimeoutDuration}, err
}

func (c *Client) GetLoggingHandler() slog.Handler {
	return sentryslog.Option{
		Level: slog.LevelError,
		AttrFromContext: []func(ctx context.Context) []slog.Attr{
			slogotel.ExtractOtelAttrFromContext([]string{}, "traceID", "spanID"),
		},
	}.NewSentryHandler()
}

func (c *Client) CleanUp(_ context.Context) error {
	flushed := sentry.Flush(c.flushDuration)
	if !flushed {
		return errors.New("failed to flush sentry")
	}
	return nil
}
