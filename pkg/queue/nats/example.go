//go:build ignore
// +build ignore

// This file provides a runnable example of using the nats package.
// To run: go run ./pkg/queue/nats/example.go
// Requires a NATS server running on localhost:4222
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	// Adjust import path according to your Go module name
	natsclient "github.com/renatoaraujo/go-monorepo-playground/pkg/queue/nats"
)

const (
	natsURL       = "nats://localhost:4222" // Default NATS server URL
	testSubject   = "greet.world"
	messageToSend = "hello modafoca"
)

func main() {
	// Setup basic logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// Setup context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Connect to NATS ---
	opts := natsclient.Options{
		URL:  natsURL,
		Name: "NATS Example Client",
	}
	client, err := natsclient.NewClient(opts)
	if err != nil {
		slog.Error("Failed to create NATS client", "error", err)
		os.Exit(1)
	}
	// Ensure connection is closed on shutdown
	go func() {
		<-ctx.Done() // Wait for shutdown signal
		slog.Info("Shutting down NATS client...")
		client.Close()
	}()

	// Use a WaitGroup to wait for the message to be received
	var wg sync.WaitGroup
	wg.Add(1) // Expect one message

	// --- Subscribe ---
	slog.Info("Subscribing to subject", "subject", testSubject)
	_, err = client.Subscribe(testSubject, func(msg *nats.Msg) {
		receivedMsg := string(msg.Data)
		slog.Info("Received message", "subject", msg.Subject, "message", receivedMsg)
		if receivedMsg == messageToSend {
			slog.Info("Correct message received!")
			wg.Done() // Signal that the expected message was received
		} else {
			slog.Warn("Received unexpected message", "expected", messageToSend, "got", receivedMsg)
			// Decide if we should still signal done or wait longer
		}
	})
	if err != nil {
		slog.Error("Failed to subscribe", "error", err)
		// client.Close() // Close client before exiting if subscribe fails critically
		os.Exit(1)
	}

	// Give subscriber a moment to be ready (optional, usually fast)
	time.Sleep(100 * time.Millisecond)

	// --- Publish ---
	slog.Info("Publishing message", "subject", testSubject, "message", messageToSend)
	err = client.Publish(testSubject, []byte(messageToSend))
	if err != nil {
		slog.Error("Failed to publish message", "error", err)
		// client.Close() // Close client before exiting if publish fails critically
		os.Exit(1)
	}

	// --- Wait for message or timeout/shutdown ---
	slog.Info("Waiting for message...")
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()       // Wait for wg.Done() to be called
		close(waitChan) // Signal that waiting is over
	}()

	select {
	case <-waitChan:
		slog.Info("Message received successfully.")
	case <-time.After(10 * time.Second): // Timeout
		slog.Error("Timeout waiting for message.")
	case <-ctx.Done(): // Shutdown signal received
		slog.Info("Shutdown signal received while waiting for message.")
	}

	// Final cleanup is handled by the deferred stop() and the goroutine calling client.Close()
	slog.Info("Example finished.")
}
