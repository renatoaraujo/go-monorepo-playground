package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/logger"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/pyroscope"
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
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	if err = setupOtelLogging(ctx, cfg, res, shutdownManager); err != nil {
		slog.ErrorContext(ctx, "failed to setup OTel logging provider", "error", err)
	}

	if err = setupTracing(ctx, cfg, res, shutdownManager); err != nil {
		slog.ErrorContext(ctx, "failed to setup tracing", "error", err)
	}

	if err = setupSentry(ctx, cfg, shutdownManager, logManager); err != nil {
		slog.ErrorContext(ctx, "failed to setup Sentry", "error", err)
	}

	if err = setupPyroscope(cfg, shutdownManager); err != nil {
		slog.ErrorContext(ctx, "failed to setup Pyroscope", "error", err)
	}

	if err = setupLogger(cfg, logManager); err != nil {
		return shutdownManager, fmt.Errorf("failed to setup logger: %w", err)
	}

	slog.InfoContext(ctx, "Observability setup complete.")
	return shutdownManager, nil
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

func setupOtelLogging(ctx context.Context, cfg *config.Config, res *resource.Resource, shutdownManager *ShutdownManager) error {
	if cfg.TracingURL == "" {
		slog.WarnContext(ctx, "OTEL_EXPORTER_OTLP_ENDPOINT not set, skipping OTLP log exporter setup for global provider.")
		return nil
	}

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
	slog.InfoContext(ctx, "global otlp logger provider configured.")
	return nil
}

func setupTracing(ctx context.Context, cfg *config.Config, res *resource.Resource, shutdownManager *ShutdownManager) error {
	if cfg.TracingEnabled {
		if cfg.TracingURL == "" {
			return fmt.Errorf("tracing enabled but OTEL_EXPORTER_OTLP_ENDPOINT is not set")
		}
		shutdown, err := tracing.Init(ctx, cfg, res)
		if err != nil {
			return fmt.Errorf("failed to setup tracing: %w", err)
		}
		if shutdown != nil {
			shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, shutdown)
		}
		slog.InfoContext(ctx, "Tracing initialized.")
	} else {
		slog.InfoContext(ctx, "Tracing is disabled by configuration.")
	}
	return nil
}

func setupSentry(ctx context.Context, cfg *config.Config, shutdownManager *ShutdownManager, logManager *LoggingManager) error {
	if cfg.SentryEnabled {
		if cfg.SentryDsn == "" {
			return fmt.Errorf("sentry enabled but SENTRY_DSN is not set")
		}
		client, err := sentry.Init(cfg)
		if err != nil {
			return fmt.Errorf("failed to setup Sentry: %w", err)
		}
		if client != nil {
			shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, client.CleanUp)
			logManager.handlers = append(logManager.handlers, client.GetLoggingHandler())
		}
		slog.InfoContext(ctx, "Sentry initialized.")
	} else {
		slog.InfoContext(ctx, "Sentry is disabled by configuration.")
	}
	return nil
}

func setupPyroscope(cfg *config.Config, shutdownManager *ShutdownManager) error {
	if cfg.PyroscopeEnabled {
		if cfg.PyroscopeURL == "" {
			return fmt.Errorf("pyroscope enabled but PYROSCOPE_URL is not set")
		}
		cleanup, err := pyroscope.Init(cfg)
		if err != nil {
			slog.ErrorContext(context.Background(), "failed to setup Pyroscope", "error", err)
			return nil
		}
		if cleanup != nil {
			shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, cleanup)
		}
		slog.InfoContext(context.Background(), "Pyroscope initialized.")
	} else {
		slog.InfoContext(context.Background(), "Pyroscope is disabled by configuration.")
	}
	return nil
}

func setupLogger(cfg *config.Config, logManager *LoggingManager) error {
	if err := logger.Init(cfg, logManager.handlers); err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	return nil
}
