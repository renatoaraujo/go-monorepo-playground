package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/logger"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/sentry"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/tracing"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type LoggingManager struct {
	handlers []slog.Handler
}

type ShutdownManager struct {
	cleanupFuncs []func(ctx context.Context) error
}

func (s *ShutdownManager) Cleanup(ctx context.Context) error {
	var err error
	cleanupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		fn := s.cleanupFuncs[i]
		err = errors.Join(err, fn(cleanupCtx))
	}
	return err
}

func Setup(ctx context.Context, cfg *config.Config) (*ShutdownManager, error) {
	var err error
	shutdownManager := &ShutdownManager{}
	logManager := &LoggingManager{}

	res, err := createOtelResource(cfg)
	if err != nil {
		return nil, err
	}

	if err = setupOtelLogging(ctx, cfg, res, shutdownManager); err != nil {
		return nil, err
	}

	if err = setupTracing(ctx, cfg, res, shutdownManager); err != nil {
		return nil, err
	}

	if err = setupSentry(cfg, shutdownManager, logManager); err != nil {
		return nil, err
	}

	if err = setupLogger(cfg, logManager); err != nil {
		return nil, err
	}

	return shutdownManager, err
}

func setupOtelLogging(ctx context.Context, cfg *config.Config, res *resource.Resource, shutdownManager *ShutdownManager) error {
	exporterCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	exporterOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.TracingURL),
		otlploggrpc.WithInsecure(),
	}
	logExporter, err := otlploggrpc.New(exporterCtx, exporterOpts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}

	logProcessor := sdklog.NewBatchProcessor(logExporter)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(logProcessor),
	)

	global.SetLoggerProvider(loggerProvider)

	shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, loggerProvider.Shutdown)

	return nil
}

func createOtelResource(cfg *config.Config) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ReleaseVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}
	return res, nil
}

func setupTracing(ctx context.Context, cfg *config.Config, res *resource.Resource, shutdownManager *ShutdownManager) error {
	if cfg.TracingEnabled {
		shutdown, err := tracing.Init(ctx, cfg, res)
		if err != nil {
			return fmt.Errorf("failed to setup tracing: %w", err)
		}
		shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, shutdown)
	}
	return nil
}

func setupSentry(cfg *config.Config, shutdownManager *ShutdownManager, logManager *LoggingManager) error {
	if cfg.SentryEnabled {
		client, err := sentry.Init(cfg)
		if err != nil {
			return fmt.Errorf(" failed to setup Sentry: %w", err)
		}
		shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, client.CleanUp)
		logManager.handlers = append(logManager.handlers, client.GetLoggingHandler())
	}
	return nil
}

func setupLogger(cfg *config.Config, logManager *LoggingManager) error {
	if err := logger.Init(cfg, logManager.handlers); err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	return nil
}
