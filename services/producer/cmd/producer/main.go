package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability"
	obsConfig "github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/queue"
	queueConfig "github.com/renatoaraujo/go-monorepo-playground/pkg/queue/config"
	"github.com/renatoaraujo/go-monorepo-playground/services/producer/internal/config"
	"github.com/renatoaraujo/go-monorepo-playground/services/producer/internal/handlers"
	"github.com/renatoaraujo/go-monorepo-playground/services/producer/internal/server"
	"github.com/renatoaraujo/go-monorepo-playground/services/producer/internal/service"
)

var (
	version     = "dev"
	serviceName = "producer"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Initialize application
	app, err := NewApp(ctx, version)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize application", "error", err)
		os.Exit(1)
	}

	// Start application
	if err = app.Run(ctx); err != nil {
		slog.ErrorContext(ctx, "application failed", "error", err)
		os.Exit(1)
	}
}

// App holds the application dependencies and configuration
type App struct {
	config           *config.Config
	server           *server.Server
	producerService  *service.ProducerService
	observabilityMgr *observability.ShutdownManager
	queueMgr         *queue.ShutdownManager
	logger           *slog.Logger
}

// NewApp creates a new application with all dependencies
func NewApp(ctx context.Context, version string) (*App, error) {
	// Load application configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Setup observability
	observabilityConfig, err := obsConfig.Init(version)
	if err != nil {
		return nil, fmt.Errorf("failed to load observability config: %w", err)
	}

	observabilityMgr, err := observability.Setup(ctx, observabilityConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup observability: %w", err)
	}

	// Get structured logger
	logger := slog.Default()

	// Setup queue (NATS)
	var publisher service.MessagePublisher
	var queueMgr *queue.ShutdownManager

	if cfg.Queue.Enabled {
		queueCfg, err := queueConfig.Init(version)
		if err != nil {
			logger.WarnContext(ctx, "failed to load queue config", "error", err)
		} else {
			natsClient, qMgr, err := queue.Setup(ctx, queueCfg)
			if err != nil {
				logger.WarnContext(ctx, "failed to setup queue", "error", err)
			} else {
				publisher = service.NewNATSClientPublisher(natsClient)
				queueMgr = qMgr
				logger.InfoContext(ctx, "queue setup successful")
			}
		}
	}

	// Create services
	producerService := service.NewProducerService(publisher, logger)

	// Create handlers
	handler := handlers.NewHandler(logger)

	// Set producer service in handler if available
	if producerService != nil {
		handler.SetProducerService(producerService, cfg.Queue.MessageSubject)
	}

	// Create HTTP server
	httpServer := server.NewServer(cfg, handler, logger, version)

	return &App{
		config:           cfg,
		server:           httpServer,
		producerService:  producerService,
		observabilityMgr: observabilityMgr,
		queueMgr:         queueMgr,
		logger:           logger,
	}, nil
}

// Run starts the application and handles graceful shutdown
func (a *App) Run(ctx context.Context) error {
	// Start publishing startup message if queue is available
	if a.config.Queue.Enabled && a.producerService != nil {
		go func() {
			// Wait a bit for connections to stabilize
			time.Sleep(2 * time.Second)

			if err := a.producerService.PublishStartupMessage(
				ctx,
				serviceName,
				version,
				a.config.Queue.StartupSubject,
			); err != nil {
				a.logger.WarnContext(ctx, "failed to publish startup message", "error", err)
			}
		}()
	}

	// Start HTTP server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := a.server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

	a.logger.InfoContext(ctx, "application started successfully",
		"service", serviceName,
		"version", version,
		"port", a.config.Server.Port,
	)

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		a.logger.InfoContext(ctx, "shutdown signal received")
	case err := <-serverErr:
		return err
	}

	// Graceful shutdown
	return a.shutdown(ctx)
}

// shutdown handles graceful shutdown of all components
func (a *App) shutdown(ctx context.Context) error {
	a.logger.InfoContext(ctx, "shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.ErrorContext(ctx, "failed to shutdown server", "error", err)
	}

	// Cleanup queue
	if a.queueMgr != nil {
		a.queueMgr.Cleanup(shutdownCtx)
	}

	// Cleanup observability
	if a.observabilityMgr != nil {
		a.observabilityMgr.Cleanup(shutdownCtx)
	}

	a.logger.InfoContext(ctx, "shutdown completed")
	return nil
}
