package nats

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Client wraps the NATS connection.
type Client struct {
	nc   *nats.Conn
	opts Options
	mu   sync.RWMutex
}

// Options holds configuration for the NATS client.
type Options struct {
	URL           string
	Name          string        // Optional connection name
	ReconnectWait time.Duration // Time to wait between reconnect attempts
	MaxReconnects int           // Max number of reconnect attempts (-1 for infinite)
	// --- NEW: Add Drain Timeout Configuration ---
	DrainTimeout time.Duration // Timeout for graceful shutdown drain operation
	// Add other nats.Option fields as needed (e.g., credentials, TLS)
}

// NewClient creates a new NATS client and connects to the server.
func NewClient(opts Options) (*Client, error) {
	// Set default options if not provided
	if opts.ReconnectWait == 0 {
		opts.ReconnectWait = 2 * time.Second
	}
	if opts.MaxReconnects == 0 {
		opts.MaxReconnects = 60
	}
	// --- NEW: Set default Drain Timeout ---
	if opts.DrainTimeout == 0 {
		opts.DrainTimeout = 10 * time.Second // Default drain timeout (adjust as needed)
	}

	nopts := []nats.Option{
		nats.Name(opts.Name),
		nats.ReconnectWait(opts.ReconnectWait),
		nats.MaxReconnects(opts.MaxReconnects),
		// --- NEW: Add DrainTimeout option here ---
		nats.DrainTimeout(opts.DrainTimeout),
		// --- End DrainTimeout addition ---
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			// Use appropriate context if available, otherwise background
			slog.Error("NATS client disconnected", "error", err, "url", nc.ConnectedUrl())
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS client reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			// Check if closed unexpectedly vs planned shutdown? nc.LastError() might help
			slog.Info("NATS client connection closed handler executed.", "last_error", nc.LastError())
		}),
		// Add other options like nats.UserCredentials() here if needed
	}

	nc, err := nats.Connect(opts.URL, nopts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", opts.URL, err)
	}

	slog.Info("Successfully connected to NATS", "url", nc.ConnectedUrl(), "name", opts.Name, "drain_timeout", opts.DrainTimeout)

	return &Client{
		nc:   nc,
		opts: opts,
	}, nil
}

// Publish sends data to the given subject.
func (c *Client) Publish(subject string, data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.nc == nil || !c.nc.IsConnected() {
		return fmt.Errorf("NATS client is not connected")
	}
	// Consider using PublishMsg for more options like headers if needed
	return c.nc.Publish(subject, data)
}

// Subscribe listens for messages on the given subject and invokes the handler.
func (c *Client) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.nc == nil || !c.nc.IsConnected() {
		return nil, fmt.Errorf("NATS client is not connected")
	}
	sub, err := c.nc.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
	}
	return sub, nil
}

// Close gracefully shuts down the NATS connection.
// It calls Drain() which respects the DrainTimeout set during connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nc != nil && !c.nc.IsClosed() { // Check if already closed
		slog.Info("Draining NATS connection...", "configured_timeout", c.opts.DrainTimeout)
		// --- MODIFIED: Use Drain() instead of DrainTimeout() ---
		// Drain respects the timeout set via nats.DrainTimeout option during connect.
		err := c.nc.Drain()
		// --- End Modification ---
		if err != nil {
			// Log the drain error. Depending on the error, the connection might still be partially open.
			slog.Error("NATS connection drain failed", "error", err)
			// As a fallback, forcefully close if drain failed? Or just log?
			// Force close might interrupt inflight messages drain was trying to handle.
			slog.Warn("Attempting forceful close after drain failure.")
			c.nc.Close() // Force close immediately
		} else {
			slog.Info("NATS connection drained successfully.")
			// Drain implicitly closes the connection upon success. Calling nc.Close() again is redundant and safe.
		}
		c.nc = nil // Mark as closed in our wrapper regardless
	} else {
		slog.Debug("NATS client Close() called but connection is already nil or closed.")
	}
}

// IsConnected checks if the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Also check nc is not nil, in case Close() was called.
	return c.nc != nil && c.nc.IsConnected()
}
