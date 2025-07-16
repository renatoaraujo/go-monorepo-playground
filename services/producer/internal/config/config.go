package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the configuration for the producer service
type Config struct {
	Server ServerConfig `json:"server"`
	Queue  QueueConfig  `json:"queue"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int `json:"port"`
	ReadTimeout  int `json:"read_timeout"`
	WriteTimeout int `json:"write_timeout"`
	IdleTimeout  int `json:"idle_timeout"`
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	Enabled        bool   `json:"enabled"`
	StartupSubject string `json:"startup_subject"`
	MessageSubject string `json:"message_subject"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvInt("SERVER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvInt("SERVER_IDLE_TIMEOUT", 120),
		},
		Queue: QueueConfig{
			Enabled:        getEnvBool("QUEUE_ENABLED", true),
			StartupSubject: getEnvString("QUEUE_STARTUP_SUBJECT", "service.startup"),
			MessageSubject: getEnvString("QUEUE_MESSAGE_SUBJECT", "messages.create"),
		},
	}

	// Load server configuration with validation
	if port := os.Getenv("SERVER_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
		}
		cfg.Server.Port = p
	}
	if rt := os.Getenv("SERVER_READ_TIMEOUT"); rt != "" {
		v, err := strconv.Atoi(rt)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_READ_TIMEOUT: %w", err)
		}
		cfg.Server.ReadTimeout = v
	}
	if wt := os.Getenv("SERVER_WRITE_TIMEOUT"); wt != "" {
		v, err := strconv.Atoi(wt)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_WRITE_TIMEOUT: %w", err)
		}
		cfg.Server.WriteTimeout = v
	}
	if it := os.Getenv("SERVER_IDLE_TIMEOUT"); it != "" {
		v, err := strconv.Atoi(it)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_IDLE_TIMEOUT: %w", err)
		}
		cfg.Server.IdleTimeout = v
	}

	// Load queue configuration
	if subject := os.Getenv("QUEUE_STARTUP_SUBJECT"); subject != "" {
		cfg.Queue.StartupSubject = subject
	}

	if enabled := os.Getenv("QUEUE_ENABLED"); enabled != "" {
		cfg.Queue.Enabled = enabled == "true"
	}

	return cfg, nil
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
