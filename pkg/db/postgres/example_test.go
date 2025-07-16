package postgres_test // Use _test package

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"testing"
	"time"

	// Use testcontainers for isolated Postgres testing
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stretchr/testify/require" // Using testify for assertions

	// Adjust import paths for your packages
	"github.com/renatoaraujo/go-monorepo-playground/pkg/db/config"            // Your config package
	dbclient "github.com/renatoaraujo/go-monorepo-playground/pkg/db/postgres" // Your postgres client package
)

// setupTestDB starts a Postgres container using testcontainers.
func setupTestDB(ctx context.Context, t *testing.T) (*postgres.PostgresContainer, config.Config) {
	t.Helper()

	// Define container request
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"), // Use a specific version
		// postgres.WithInitScripts(filepath.Join("testdata", "init-db.sql")), // Optional: Run init scripts
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("test-user"),
		postgres.WithPassword("test-password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).                 // Wait for the log msg twice for stability
				WithStartupTimeout(1*time.Minute), // Generous timeout for container start
		),
	)
	require.NoError(t, err, "Failed to start postgres container")

	// Get connection details from the running container
	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)
	port, err := pgContainer.MappedPort(ctx, "5432/tcp") // Use mapped port
	require.NoError(t, err)

	// Create a config matching the test container
	testCfg := config.Config{
		DbEnabled:         true,
		DbHost:            host,
		DbPort:            uint16(port.Int()), // Convert testcontainers port to uint16
		DbUser:            "test-user",
		DbPassword:        "test-password",
		DbName:            "test-db",
		DbSSLMode:         "disable", // SSL typically disabled for local test containers
		DbMaxConns:        5,         // Lower limits for testing
		DbMinConns:        1,
		DbMaxConnLifetime: 5 * time.Minute,
		DbMaxConnIdleTime: 1 * time.Minute,
		DbConnectTimeout:  10 * time.Second, // Increased timeout for test container setup
	}

	t.Logf("Test PostgreSQL container started: host=%s, port=%d", testCfg.DbHost, testCfg.DbPort)

	return pgContainer, testCfg
}

func TestClient_ConnectionAndPing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode.")
	}
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testcontainers test in CI environment unless Docker is configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Overall test timeout
	defer cancel()

	pgContainer, testCfg := setupTestDB(ctx, t)
	// Ensure container is terminated cleanly after the test
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("WARN: failed to terminate test container: %v", err)
		}
	}()

	// Create client using the test config
	client, err := dbclient.NewClient(ctx, testCfg)
	require.NoError(t, err, "NewClient should connect successfully to test DB")
	require.NotNil(t, client, "Client should not be nil")
	defer client.Close() // Ensure client pool is closed

	// Test Ping method
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	err = client.Ping(pingCtx)
	require.NoError(t, err, "Client ping should succeed")

	t.Log("Connection and Ping test successful.")
}

func TestClient_ExecAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode.")
	}
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testcontainers test in CI environment unless Docker is configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Overall test timeout
	defer cancel()

	pgContainer, testCfg := setupTestDB(ctx, t)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("WARN: failed to terminate test container: %v", err)
		}
	}()

	client, err := dbclient.NewClient(ctx, testCfg)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	// 1. Create Table (Exec)
	createTableSQL := `CREATE TABLE test_items (id SERIAL PRIMARY KEY, name TEXT NOT NULL)`
	cmdTag, err := client.Exec(ctx, createTableSQL)
	require.NoError(t, err, "Exec CREATE TABLE failed")
	t.Logf("CREATE TABLE command tag: %s", cmdTag) // Should indicate CREATE TABLE

	// 2. Insert Row (Exec)
	itemName := fmt.Sprintf("Widget %d", time.Now().UnixNano())
	insertSQL := `INSERT INTO test_items (name) VALUES ($1)`
	cmdTag, err = client.Exec(ctx, insertSQL, itemName)
	require.NoError(t, err, "Exec INSERT failed")
	require.EqualValues(t, 1, cmdTag.RowsAffected(), "INSERT should affect 1 row")
	t.Logf("INSERT command tag: %s", cmdTag) // Should indicate INSERT 0 1

	// 3. Query Row (QueryRow + Scan)
	var retrievedName string
	var retrievedID int
	queryRowSQL := `SELECT id, name FROM test_items WHERE name = $1`
	row := client.QueryRow(ctx, queryRowSQL, itemName)
	err = row.Scan(&retrievedID, &retrievedName)
	require.NoError(t, err, "QueryRow Scan failed")
	require.True(t, retrievedID > 0, "Retrieved ID should be positive")
	require.Equal(t, itemName, retrievedName, "Retrieved name should match inserted name")
	t.Logf("QueryRow successful: ID=%d, Name=%s", retrievedID, retrievedName)

	// 4. Insert another row and Query multiple (Query + CollectRows)
	itemName2 := fmt.Sprintf("Gadget %d", time.Now().UnixNano())
	cmdTag, err = client.Exec(ctx, insertSQL, itemName2)
	require.NoError(t, err, "Exec INSERT (2nd item) failed")
	require.EqualValues(t, 1, cmdTag.RowsAffected(), "INSERT (2nd item) should affect 1 row")

	type TestItem struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	querySQL := `SELECT id, name FROM test_items ORDER BY id ASC`
	rows, err := client.Query(ctx, querySQL)
	require.NoError(t, err, "Query multiple rows failed")
	// Use RowToStructByName helper
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[TestItem])
	require.NoError(t, err, "CollectRows failed")
	require.Len(t, items, 2, "Should have retrieved 2 items")
	require.Equal(t, itemName, items[0].Name)
	require.Equal(t, itemName2, items[1].Name)
	t.Logf("Query multiple successful: Found %d items", len(items))
}
