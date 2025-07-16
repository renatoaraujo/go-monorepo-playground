package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	// Assumed location for your queue config
	"github.com/renatoaraujo/go-monorepo-playground/pkg/queue/config"
	// Import the nats client package we created
	natsclient "github.com/renatoaraujo/go-monorepo-playground/pkg/queue/nats"
)

// ShutdownManager handles the graceful shutdown of queue clients.
type ShutdownManager struct {
	cleanupFuncs []func(ctx context.Context) error
}

// Cleanup runs all registered cleanup functions.
func (s *ShutdownManager) Cleanup(ctx context.Context) error {
	var err error
	// Use a reasonable timeout for queue cleanup (e.g., NATS drain)
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Execute cleanup functions in reverse order of addition
	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		fn := s.cleanupFuncs[i]
		// Assign the joined error back to err in each iteration
		err = errors.Join(err, fn(cleanupCtx)) // errors.Join handles nil errors correctly
	}
	return err
}

// Setup initializes queue clients based on configuration.
// Currently, it only sets up NATS. It returns the NATS client instance,
// the ShutdownManager, and any error during setup.
func Setup(ctx context.Context, cfg *config.Config) (*natsclient.Client, *ShutdownManager, error) {
	shutdownManager := &ShutdownManager{}
	var natsCl *natsclient.Client // Variable to hold the NATS client
	var err error

	// --- Setup NATS Client ---
	if cfg.NatsEnabled {
		slog.InfoContext(ctx, "NATS is enabled, attempting setup...")
		natsCl, err = setupNats(cfg, shutdownManager)
		if err != nil {
			// Decide if NATS setup failure is fatal for the application
			slog.ErrorContext(ctx, "Failed to setup NATS client", "error", err)
			// return nil, shutdownManager, fmt.Errorf("failed to setup NATS: %w", err) // Option: return error to halt startup
			// Option: Continue without NATS if possible, natsCl will be nil
		} else {
			slog.InfoContext(ctx, "NATS client setup successfully.")
		}
	} else {
		slog.InfoContext(ctx, "NATS is disabled by configuration.")
	}

	// --- Setup Other Queue Systems (e.g., Kafka, RabbitMQ) ---
	// Add calls to their setup functions here later, potentially returning
	// multiple client types or a generic queue interface.
	// For now, we only return the NATS client.

	// If no clients were successfully initialized and setup is considered successful overall (e.g., NATS was disabled)
	if natsCl == nil && err == nil {
		slog.InfoContext(ctx, "No queue clients initialized.")
	}

	// Reset non-critical errors to nil before returning
	if err != nil {
		slog.WarnContext(ctx, "Non-critical errors occurred during setup but will not block startup:", "error", err)
		err = nil
	}

	// Return the (potentially nil) NATS client, the shutdown manager, and any critical setup error
	return natsCl, shutdownManager, err
}

// setupNats initializes the NATS client using the config.
func setupNats(cfg *config.Config, sm *ShutdownManager) (*natsclient.Client, error) {
	// Map queue/config.Config fields to natsclient.Options
	natsOpts := natsclient.Options{
		URL:           cfg.NatsURL,
		Name:          cfg.NatsClientName, // Use the name from config
		ReconnectWait: cfg.NatsReconnectWait,
		MaxReconnects: cfg.NatsMaxReconnects,
		// Map other options like credentials if added to config
	}

	// Create the NATS client using the dedicated nats package
	client, err := natsclient.NewClient(natsOpts)
	if err != nil {
		return nil, fmt.Errorf("natsclient.NewClient failed: %w", err)
	}

	// Register the NATS client's Close method for graceful shutdown
	if client != nil {
		cleanupFunc := func(_ context.Context) error {
			slog.Info("Closing NATS client connection...")
			// natsclient.Close() handles drain logic internally now
			client.Close()
			// Currently, the underlying nats.Conn.Close/Drain doesn't easily return errors here
			// We rely on logging within the Close method.
			return nil
		}
		sm.cleanupFuncs = append(sm.cleanupFuncs, cleanupFunc)
	}

	return client, nil
}
