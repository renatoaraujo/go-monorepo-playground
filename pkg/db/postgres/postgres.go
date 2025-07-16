package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	// Assuming config struct is defined here or passed in
	// Use your actual config import path
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/config" // Example path
)

// PgxClient wraps the pgx connection pool.
type PgxClient struct {
	pool *pgxpool.Pool
	cfg  config.Config // Store config for reference if needed
}

// NewClient creates a new PostgreSQL client with a connection pool.
func NewClient(ctx context.Context, cfg config.Config) (*PgxClient, error) {
	connString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DbHost, cfg.DbPort, cfg.DbUser, cfg.DbPassword, cfg.DbName, cfg.DbSSLMode)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres connection string: %w", err)
	}

	// Apply pool settings from config
	poolConfig.MaxConns = cfg.DbMaxConns
	poolConfig.MinConns = cfg.DbMinConns
	poolConfig.MaxConnLifetime = cfg.DbMaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DbMaxConnIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.DbConnectTimeout

	// Health check runs periodically in the background
	// poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Optional: Configure logging for pgx actions
	// poolConfig.ConnConfig.Logger = pgxlogadapter.NewLogger(slog.Default()) // Requires github.com/jackc/pgx-slog
	// poolConfig.ConnConfig.LogLevel = pgxLogLevelFromSlog(slog.Default().Level()) // Need mapping func

	connectCtx, cancel := context.WithTimeout(ctx, cfg.DbConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	// Ping the database to ensure connectivity during initialization
	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.DbConnectTimeout)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // Close pool if ping fails
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	slog.InfoContext(ctx, "Successfully connected to PostgreSQL",
		slog.String("host", cfg.DbHost),
		slog.Int("port", int(cfg.DbPort)),
		slog.String("database", cfg.DbName),
		slog.String("user", cfg.DbUser))

	return &PgxClient{
		pool: pool,
		cfg:  cfg,
	}, nil
}

// Close closes the connection pool.
func (c *PgxClient) Close() {
	if c.pool != nil {
		slog.Info("Closing PostgreSQL connection pool...")
		c.pool.Close()
		slog.Info("PostgreSQL connection pool closed.")
	}
}

// Pool returns the underlying connection pool for more advanced use cases
// like transactions or direct pool access if needed.
func (c *PgxClient) Pool() *pgxpool.Pool {
	return c.pool
}

// Ping verifies the connection to the database.
func (c *PgxClient) Ping(ctx context.Context) error {
	if c.pool == nil {
		return fmt.Errorf("postgres client pool is not initialized")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Use a reasonable timeout for ping
	defer cancel()
	return c.pool.Ping(pingCtx)
}

// --- Convenience Wrappers (optional but often helpful) ---

// Exec executes a command (like INSERT, UPDATE, DELETE) and returns the result tag.
func (c *PgxClient) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if c.pool == nil {
		return pgconn.CommandTag{}, fmt.Errorf("postgres client pool is not initialized")
	}
	return c.pool.Exec(ctx, sql, args...)
}

// Query executes a query that returns multiple rows.
// Use pgx.CollectRows or iterate over the returned Rows object.
// Remember to call Rows.Close() when done, often via `defer rows.Close()`.
func (c *PgxClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if c.pool == nil {
		// Returning an error might be better than nil Rows here.
		// pgxpool.ErrNotConnected might be appropriate? Or a custom error.
		// For now, return error, caller must check. pgx.Rows is an interface.
		return nil, fmt.Errorf("postgres client pool is not initialized")
	}
	return c.pool.Query(ctx, sql, args...)
}

// QueryRow executes a query expected to return at most one row.
// Use the Scan method on the returned Row object.
func (c *PgxClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if c.pool == nil {
		slog.ErrorContext(ctx, "QueryRow called on uninitialized postgres client pool")
		return &ErrorRow{err: fmt.Errorf("postgres client pool is not initialized")}
	}
	return c.pool.QueryRow(ctx, sql, args...)
}

// --- Add Transaction helper example (optional) ---
// BeginTx starts a new transaction.
// func (c *PgxClient) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
// 	return c.pool.BeginTx(ctx, opts)
// }
