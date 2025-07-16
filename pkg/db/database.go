package db

import (
	"context"
	"errors"
	"log/slog"
	"time"

	// --- Adjust these import paths ---
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/config"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/postgres" // Import the postgres client package
	// --- End path adjustments ---
)

// ShutdownManager handles the graceful shutdown of database clients.
type ShutdownManager struct {
	cleanupFuncs []func(ctx context.Context) error
}

// Cleanup runs all registered cleanup functions for database clients.
func (s *ShutdownManager) Cleanup(ctx context.Context) error {
	var err error
	// Use a reasonable timeout for DB cleanup
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	slog.InfoContext(cleanupCtx, "Starting database cleanup...")

	// Execute cleanup functions in reverse order of addition
	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		fn := s.cleanupFuncs[i]
		slog.DebugContext(cleanupCtx, "Running database cleanup function", "index", i)
		err = errors.Join(err, fn(cleanupCtx))
	}

	if err != nil {
		slog.ErrorContext(cleanupCtx, "Database cleanup completed with errors", "error", err)
	} else {
		slog.InfoContext(cleanupCtx, "Database cleanup completed successfully.")
	}
	return err
}

// Setup acts as the main entry point to initialize database clients.
// Currently only supports PostgreSQL via pgx.
//
// Returns:
//   - *postgres.PgxClient: Instance of the initialized PostgreSQL client (or nil).
//   - *ShutdownManager:   Manages cleanup functions.
//   - error:              Any critical setup error.
func Setup(ctx context.Context, cfg *config.Config) (*postgres.PgxClient, *ShutdownManager, error) {
	shutdownManager := &ShutdownManager{}
	var pgClient *postgres.PgxClient // Variable to hold the Postgres client instance
	var setupErr error

	slog.InfoContext(ctx, "Starting database setup...")

	// --- Setup PostgreSQL Client ---
	if cfg.DbEnabled {
		slog.InfoContext(ctx, "PostgreSQL is enabled in config, attempting setup...")

		// Call the NewClient function from the postgres package
		client, err := postgres.NewClient(ctx, *cfg) // Pass the db config section
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize PostgreSQL client", "error", err)
			// Decide if failure is critical
			// setupErr = fmt.Errorf("failed to setup PostgreSQL: %w", err)
			// return nil, shutdownManager, setupErr
			pgClient = nil // Ensure client is nil
		} else {
			slog.InfoContext(ctx, "PostgreSQL client setup successful.")
			pgClient = client // Assign successful client

			// Register the client's Close method for graceful shutdown
			cleanupFunc := func(_ context.Context) error {
				// The context passed to Cleanup isn't directly used by pool.Close()
				slog.Info("Closing PostgreSQL client connection pool via cleanup manager...")
				pgClient.Close()
				return nil // pgxpool.Pool.Close() doesn't return an error
			}
			shutdownManager.cleanupFuncs = append(shutdownManager.cleanupFuncs, cleanupFunc)
			slog.DebugContext(ctx, "Registered PostgreSQL client cleanup function.")
		}
	} else {
		slog.InfoContext(ctx, "PostgreSQL is disabled by configuration.")
	}

	// --- Setup Other Database Types ---
	// Add setup for MySQL, MongoDB, etc. here if needed

	// --- Finalize Setup ---
	if pgClient == nil && setupErr == nil {
		slog.InfoContext(ctx, "Database setup finished, no clients initialized.")
	} else if setupErr != nil {
		slog.ErrorContext(ctx, "Database setup finished with critical errors.", "error", setupErr)
	} else {
		slog.InfoContext(ctx, "Database setup finished successfully.")
	}

	// Return the initialized client(s), manager, and any critical error
	return pgClient, shutdownManager, setupErr
}
