package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

var version string

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.Init(version)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load configuration", "error", err)
	}

	shutdownManager, err := observability.Setup(ctx, cfg)
	if err != nil {
		slog.ErrorContext(ctx, "failed to setup observability", "error", err)
		shutdownManager.Cleanup(ctx)
		os.Exit(1)
	}
	defer shutdownManager.Cleanup(ctx)

	uk := attribute.Key("username")

	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		ctx = req.Context()
		span := trace.SpanFromContext(ctx)
		bag := baggage.FromContext(ctx)
		span.AddEvent("handling this...", trace.WithAttributes(uk.String(bag.Member("username").Value())))
		w.Header().Set("Content-Type", "application/json")

		_, err = w.Write([]byte(`{"message": "Hello, World!"}`))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.ErrorContext(ctx, "failed to write response", "error", err)
			return
		}
	}

	otelHandler := otelhttp.NewHandler(http.HandlerFunc(helloHandler), "Hello")

	http.Handle("/", otelHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: otelHandler,
	}

	go func() {
		if err = server.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			slog.ErrorContext(ctx, "could not listen on port 8080", "error", err)
		}
	}()

	<-ctx.Done()

	slog.InfoContext(ctx, "shutting down gracefully, press Ctrl+C again to force")

	if err = server.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server forced to shutdown", "error", err)
	}
}
