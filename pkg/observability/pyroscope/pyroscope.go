package pyroscope

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/renatoaraujo/go-monorepo-playground/pkg/observability/config"

	"github.com/grafana/pyroscope-go"
)

// Init initializes the Pyroscope continuous profiler based on the provided configuration.
// It returns a cleanup function to be called on application shutdown.
func Init(cfg *config.Config) (func(ctx context.Context) error, error) {
	// Create a no-op cleanup function by default
	cleanup := func(ctx context.Context) error { return nil }

	// Map string config values to Pyroscope ProfileType constants
	profileTypes := mapProfileTypes(cfg.PyroscopeProfileTypes)
	if len(profileTypes) == 0 {
		// If no specific types are configured, maybe default to CPU? Or log a warning?
		// For now, let's default to CPU if none are specified or mapping fails
		profileTypes = []pyroscope.ProfileType{pyroscope.ProfileCPU}
		// Consider logging this default action:
		// slog.Warn("No valid Pyroscope profile types configured or found, defaulting to CPU only.")
	}

	// Set runtime profile rates if mutex/block profiling is enabled via config
	// Note: These are global settings affecting the entire Go runtime.
	if cfg.PyroscopeEnableMutexProfiling {
		runtime.SetMutexProfileFraction(cfg.PyroscopeMutexProfileFraction)
		// slog.Info("Mutex profiling enabled", "fraction", cfg.PyroscopeMutexProfileFraction)
	}
	if cfg.PyroscopeEnableBlockProfiling {
		runtime.SetBlockProfileRate(cfg.PyroscopeBlockProfileRate)
		// slog.Info("Block profiling enabled", "rate", cfg.PyroscopeBlockProfileRate)
	}

	// Start the profiler
	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: cfg.ServiceName, // IMPORTANT: Use the same ServiceName as OTel for correlation
		ServerAddress:   cfg.PyroscopeURL,
		// Logger: pyroscope.StandardLogger // Optional: Wire your slog logger if desired
		ProfileTypes: profileTypes,
		// Optional: Add tags relevant to your service instance if needed
		// Tags:            map[string]string{"region": "us-west-1"},
	})

	if err != nil {
		return cleanup, fmt.Errorf("failed to start pyroscope profiler: %w", err)
	}

	// If profiler started successfully, return its Stop method as the cleanup function.
	cleanup = func(_ context.Context) error {
		// Pyroscope's Stop doesn't return an error directly in the signature used here.
		// It might log errors internally if configured with a logger.
		return profiler.Stop()
	}

	// Consider logging successful initialization:
	// slog.Info("Pyroscope profiler initialized", "url", cfg.PyroscopeURL, "service", cfg.ServiceName)

	return cleanup, nil
}

// mapProfileTypes converts a slice of strings to Pyroscope ProfileType constants.
func mapProfileTypes(types []string) []pyroscope.ProfileType {
	mapped := make([]pyroscope.ProfileType, 0, len(types))
	knownTypes := map[string]pyroscope.ProfileType{
		"cpu":            pyroscope.ProfileCPU,
		"alloc_objects":  pyroscope.ProfileAllocObjects,
		"alloc_space":    pyroscope.ProfileAllocSpace,
		"inuse_objects":  pyroscope.ProfileInuseObjects,
		"inuse_space":    pyroscope.ProfileInuseSpace,
		"goroutines":     pyroscope.ProfileGoroutines,
		"mutex_count":    pyroscope.ProfileMutexCount,
		"mutex_duration": pyroscope.ProfileMutexDuration,
		"block_count":    pyroscope.ProfileBlockCount,
		"block_duration": pyroscope.ProfileBlockDuration,
	}

	for _, t := range types {
		normalized := strings.ToLower(strings.TrimSpace(t))
		if pt, ok := knownTypes[normalized]; ok {
			mapped = append(mapped, pt)
		} else {
			// Log unknown type?
			// slog.Warn("Unknown Pyroscope profile type configured", "type", t)
		}
	}
	return mapped
}
