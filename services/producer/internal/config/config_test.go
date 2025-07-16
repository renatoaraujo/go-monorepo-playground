package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name:    "default configuration",
			envVars: map[string]string{},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 8080 {
					t.Errorf("expected port 8080, got %d", cfg.Server.Port)
				}
				if cfg.Queue.StartupSubject != "service.startup" {
					t.Errorf("expected startup subject 'service.startup', got %s", cfg.Queue.StartupSubject)
				}
				if !cfg.Queue.Enabled {
					t.Error("expected queue to be enabled by default")
				}
			},
		},
		{
			name: "custom server port",
			envVars: map[string]string{
				"SERVER_PORT": "9090",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 9090 {
					t.Errorf("expected port 9090, got %d", cfg.Server.Port)
				}
			},
		},
		{
			name: "custom timeouts",
			envVars: map[string]string{
				"SERVER_READ_TIMEOUT":  "60",
				"SERVER_WRITE_TIMEOUT": "60",
				"SERVER_IDLE_TIMEOUT":  "240",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Server.ReadTimeout != 60 {
					t.Errorf("expected read timeout 60, got %d", cfg.Server.ReadTimeout)
				}
				if cfg.Server.WriteTimeout != 60 {
					t.Errorf("expected write timeout 60, got %d", cfg.Server.WriteTimeout)
				}
				if cfg.Server.IdleTimeout != 240 {
					t.Errorf("expected idle timeout 240, got %d", cfg.Server.IdleTimeout)
				}
			},
		},
		{
			name: "custom queue configuration",
			envVars: map[string]string{
				"QUEUE_STARTUP_SUBJECT": "custom.startup",
				"QUEUE_ENABLED":         "false",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Queue.StartupSubject != "custom.startup" {
					t.Errorf("expected startup subject 'custom.startup', got %s", cfg.Queue.StartupSubject)
				}
				if cfg.Queue.Enabled {
					t.Error("expected queue to be disabled")
				}
			},
		},
		{
			name: "invalid server port",
			envVars: map[string]string{
				"SERVER_PORT": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			envVars: map[string]string{
				"SERVER_READ_TIMEOUT": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if cfg == nil {
				t.Error("expected config but got nil")
				return
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadWithEnvironmentCleanup(t *testing.T) {
	// Test that environment variables don't affect other tests
	os.Setenv("SERVER_PORT", "9999")
	defer os.Unsetenv("SERVER_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}
}
