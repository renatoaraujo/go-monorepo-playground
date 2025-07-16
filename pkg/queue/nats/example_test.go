package nats_test // Use _test package to test public API

import (
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	// Adjust import path according to your Go module name
	natsclient "github.com/renatoaraujo/go-monorepo-playground/pkg/queue/nats"
	"github.com/stretchr/testify/require" // Using testify for assertions
)

const (
	testSubject = "test.greet.world"
	testMessage = "hello modafoca"
	testTimeout = 5 * time.Second // Timeout for the test operations
)

// Helper function to start an embedded NATS server for testing.
func runNatsServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	// Use a random port to avoid conflicts
	opts.Port = -1
	opts.Host = "127.0.0.1"
	opts.LogFile = "" // Disable file logging for tests
	opts.NoLog = true // Disable console logging unless debugging
	return natsserver.RunServer(&opts)
}

func TestPublishSubscribe(t *testing.T) {
	// Start embedded NATS server
	s := runNatsServer(t)
	defer s.Shutdown() // Ensure server is shut down after the test

	// Configure client options to connect to the test server
	opts := natsclient.Options{
		URL:           s.ClientURL(), // Get URL from the running test server
		Name:          "Test NATS Client",
		ReconnectWait: 100 * time.Millisecond, // Faster reconnect for tests
		MaxReconnects: 5,
	}

	// Create client instance
	client, err := natsclient.NewClient(opts)
	require.NoError(t, err, "Failed to create NATS client")
	require.NotNil(t, client, "Client should not be nil")
	defer client.Close() // Ensure client connection is closed

	// Use WaitGroup and channel for synchronization
	var wg sync.WaitGroup
	wg.Add(1)
	msgReceived := make(chan *nats.Msg, 1) // Buffered channel

	// Subscribe
	sub, err := client.Subscribe(testSubject, func(msg *nats.Msg) {
		// Send received message to channel for assertion in main test goroutine
		// Avoid blocking in the callback
		select {
		case msgReceived <- msg:
		default:
			// Should not happen with buffered channel if wg logic is correct
			t.Logf("Warning: Message channel full or closed unexpectedly")
		}
		wg.Done() // Signal that a message was processed
	})
	require.NoError(t, err, "Failed to subscribe")
	require.NotNil(t, sub, "Subscription should not be nil")
	defer sub.Unsubscribe() // Clean up subscription

	// Ensure subscription is processed by the server
	// Adding a small delay or using nats connection Flush might be necessary in complex scenarios
	time.Sleep(50 * time.Millisecond) // Small grace period

	// Publish
	err = client.Publish(testSubject, []byte(testMessage))
	require.NoError(t, err, "Failed to publish message")

	// Wait for the message or timeout
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()       // Wait for handler to call Done()
		close(waitChan) // Signal completion
	}()

	select {
	case <-waitChan:
		// Wait finished, now check the received message from the channel
		select {
		case received := <-msgReceived:
			require.Equal(t, testSubject, received.Subject, "Received message subject mismatch")
			require.Equal(t, testMessage, string(received.Data), "Received message data mismatch")
			t.Logf("Successfully received and verified message: %q on subject %q", string(received.Data), received.Subject)
		default:
			// This case should ideally not be hit if wg.Wait completed correctly
			t.Fatal("WaitGroup finished but no message received on channel")
		}
	case <-time.After(testTimeout):
		t.Fatalf("Timeout (%s) waiting for message on subject %s", testTimeout, testSubject)
	}
}

func TestClient_IsNotConnectedInitially(t *testing.T) {
	// Test without starting server to ensure connection fails
	opts := natsclient.Options{
		URL:           "nats://localhost:12345", // Use a port likely not in use
		MaxReconnects: 1,                        // Try only once quickly
		ReconnectWait: 10 * time.Millisecond,
	}
	_, err := natsclient.NewClient(opts)
	require.Error(t, err, "Expected connection error for non-existent server")
}
