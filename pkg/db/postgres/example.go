//go:build ignore
// +build ignore

// This file provides a runnable example of using the postgres package.
// To run: go run ./pkg/db/postgres/example.go
// Requires environment variables for DB connection (DB_HOST, DB_PORT, etc.)
// or update the default values in config.Init()
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5" // Import for pgx types like pgx.RowToStructByName
	// Adjust import paths
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/config"
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/postgres" // Import the client package
)

func main() {
	// Setup basic logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	ctx := context.Background()

	// --- Load Config ---
	// Assumes config.Init reads from Env Vars or has usable defaults
	cfg, err := config.Init()
	if err != nil {
		slog.Error("Failed to load db config", "error", err)
		os.Exit(1)
	}

	if !cfg.DbEnabled {
		slog.Info("Database is disabled in config. Exiting example.")
		os.Exit(0)
	}

	// --- Connect to PostgreSQL ---
	client, err := postgres.NewClient(ctx, *cfg)
	if err != nil {
		slog.Error("Failed to create PostgreSQL client", "error", err)
		os.Exit(1)
	}
	defer client.Close() // Ensure pool is closed on exit

	slog.Info("Client created. Pinging database...")
	if err := client.Ping(ctx); err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("Ping successful!")

	// --- Perform DB Operations ---
	err = setupSchema(ctx, client)
	if err != nil {
		slog.Error("Failed to set up schema", "error", err)
		os.Exit(1)
	}

	err = insertRecord(ctx, client, "Alice", "alice@example.com")
	if err != nil {
		slog.Error("Failed to insert record", "error", err)
		os.Exit(1)
	}
	id, err := insertRecordReturningID(ctx, client, "Bob", "bob@example.com")
	if err != nil {
		slog.Error("Failed to insert record returning ID", "error", err)
		os.Exit(1)
	}
	slog.Info("Inserted Bob with ID", "id", id)

	err = queryRecords(ctx, client)
	if err != nil {
		slog.Error("Failed to query records", "error", err)
		os.Exit(1)
	}

	slog.Info("Example finished successfully.")
}

func setupSchema(ctx context.Context, client *postgres.PgxClient) error {
	slog.Info("Setting up schema (CREATE TABLE IF NOT EXISTS)...")
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);`
	_, err := client.Exec(ctx, createTableSQL)
	return err
}

func insertRecord(ctx context.Context, client *postgres.PgxClient, name, email string) error {
	slog.Info("Inserting record...", "name", name, "email", email)
	insertSQL := `INSERT INTO users (name, email) VALUES ($1, $2)`
	cmdTag, err := client.Exec(ctx, insertSQL, name, email)
	if err != nil {
		return fmt.Errorf("exec insert failed: %w", err)
	}
	if cmdTag.RowsAffected() != 1 {
		return fmt.Errorf("expected 1 row to be affected, but got %d", cmdTag.RowsAffected())
	}
	slog.Info("Record inserted successfully.")
	return nil
}

func insertRecordReturningID(ctx context.Context, client *postgres.PgxClient, name, email string) (int, error) {
	slog.Info("Inserting record returning ID...", "name", name, "email", email)
	insertSQL := `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`
	var returnedID int
	// Use QueryRow for statements expected to return one row (like RETURNING id)
	row := client.QueryRow(ctx, insertSQL, name, email)
	err := row.Scan(&returnedID) // Scan the returned ID into the variable
	if err != nil {
		return 0, fmt.Errorf("queryrow insert failed: %w", err)
	}
	slog.Info("Record inserted successfully with ID.", "id", returnedID)
	return returnedID, nil
}

type User struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
}

func queryRecords(ctx context.Context, client *postgres.PgxClient) error {
	slog.Info("Querying records from users table...")
	querySQL := `SELECT id, name, email, created_at FROM users ORDER BY created_at DESC LIMIT 10`

	rows, err := client.Query(ctx, querySQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close() // IMPORTANT: Close rows when done iterating

	// Example 1: Scan row by row
	slog.Info("--- Users (Scanning Row by Row) ---")
	for rows.Next() {
		var user User
		// Scan into fields directly
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		slog.Info("User Found", "id", user.ID, "name", user.Name, "email", user.Email, "created_at", user.CreatedAt.Format(time.RFC3339))
	}
	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error during rows iteration: %w", err)
	}

	// Example 2: Collect all rows into a slice (useful for smaller result sets)
	// Re-run query or use a different query for this example
	slog.Info("--- Users (Collecting All Rows) ---")
	rows, err = client.Query(ctx, querySQL) // Re-run query
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	// pgx.CollectRows is generic, RowToStructByName helps map columns to struct fields by name
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[User])
	if err != nil {
		return fmt.Errorf("failed to collect rows: %w", err)
	}

	if len(users) == 0 {
		slog.Info("No users found via CollectRows.")
	} else {
		for _, user := range users {
			slog.Info("User Found (Collected)", "id", user.ID, "name", user.Name, "email", user.Email, "created_at", user.CreatedAt.Format(time.RFC3339))
		}
	}

	slog.Info("Querying finished.")
	return nil
}
