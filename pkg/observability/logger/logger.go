package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"

	slogotel "github.com/remychantenay/slog-otel"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func Init(cfg *config.Config, handlers []slog.Handler) error {
	var level slog.Level
	leveler := &slog.LevelVar{}

	switch strings.ToLower(cfg.LoggingLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return fmt.Errorf("unsupported log level: %s", cfg.LoggingLevel)
	}
	leveler.Set(level)

	addSource := false
	handlerOpts := &slog.HandlerOptions{
		Level:     leveler,
		AddSource: addSource,
	}

	var consoleHandler slog.Handler
	switch strings.ToLower(cfg.LoggingFormat) {
	case "text":
		consoleHandler = slog.NewTextHandler(os.Stdout, handlerOpts)
	case "json":
		consoleHandler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	default:
		consoleHandler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}

	otelHandlerOpts := []otelslog.Option{
		otelslog.WithSource(addSource),
		otelslog.WithVersion(cfg.ReleaseVersion),
		otelslog.WithSchemaURL(semconv.SchemaURL),
	}
	otelHandler := otelslog.NewHandler(cfg.ServiceName+"/logger", otelHandlerOpts...)

	allOutputHandlers := append([]slog.Handler{}, handlers...)
	allOutputHandlers = append(allOutputHandlers, consoleHandler)
	allOutputHandlers = append(allOutputHandlers, otelHandler)

	handlerWithOtelContext := slogotel.OtelHandler{
		Next: slogmulti.Fanout(allOutputHandlers...),
	}

	slog.SetDefault(slog.New(handlerWithOtelContext))

	slog.LogAttrs(context.Background(), slog.LevelInfo, "Logger initialised",
		slog.String("log_level", level.String()),
		slog.String("log_format", cfg.LoggingFormat),
	)

	slog.Info("logger initialised")

	return nil
}
