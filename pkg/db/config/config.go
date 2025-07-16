package config

import (
	"fmt"
	"time"
	// Add config loading library imports if used (e.g., envconfig)
)

// Config holds database-related configuration settings.
type Config struct {
	// --- PostgreSQL Configuration ---
	DbEnabled         bool          `envconfig:"DB_ENABLED" default:"true"`
	DbHost            string        `envconfig:"DB_HOST" default:"localhost"`
	DbPort            uint16        `envconfig:"DB_PORT" default:"5432"`
	DbUser            string        `envconfig:"DB_USER" default:"user"`
	DbPassword        string        `envconfig:"DB_PASSWORD" default:"password"`
	DbName            string        `envconfig:"DB_NAME" default:"mydatabase"`
	DbSSLMode         string        `envconfig:"DB_SSLMODE" default:"disable"` // or "require", "verify-full", etc.
	DbMaxConns        int32         `envconfig:"DB_MAX_CONNS" default:"10"`
	DbMinConns        int32         `envconfig:"DB_MIN_CONNS" default:"2"`
	DbMaxConnLifetime time.Duration `envconfig:"DB_MAX_CONN_LIFETIME" default:"1h"`
	DbMaxConnIdleTime time.Duration `envconfig:"DB_MAX_CONN_IDLE_TIME" default:"30m"`
	DbConnectTimeout  time.Duration `envconfig:"DB_CONNECT_TIMEOUT" default:"5s"`

	// Add DSN as an alternative if preferred
	// DbDSN             string        `envconfig:"DB_DSN"`

	// Add configs for other DB types here later if needed
}

// Init loads the database configuration.
// Implement this based on your project's config loading strategy.
func Init() (*Config, error) {
	cfg := &Config{}

	// --- Placeholder for your config loading logic ---
	// Example using envconfig:
	// if err := envconfig.Process("", cfg); err != nil {
	//     return nil, fmt.Errorf("failed to process db env config: %w", err)
	// }

	// Load env vars manually if not using a library
	// cfg.DbHost = getEnv("DB_HOST", cfg.DbHost) ... etc.

	// Post-load validation
	if cfg.DbEnabled {
		if cfg.DbHost == "" || cfg.DbUser == "" || cfg.DbName == "" {
			return nil, fmt.Errorf("database is enabled but host, user, or dbname is missing")
		}
		// Validate DbSSLMode maybe?
	}

	return cfg, nil
}

// Add helper like getEnv if loading manually
// import "os"
// func getEnv(key, fallback string) string { ... }
