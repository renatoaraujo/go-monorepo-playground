package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	natsclient "github.com/renatoaraujo/go-monorepo-playground/pkg/queue/nats"
)

// MessagePublisher defines the interface for publishing messages
type MessagePublisher interface {
	Publish(subject string, data []byte) error
}

// NATSClientPublisher implements MessagePublisher using NATS client
type NATSClientPublisher struct {
	client *natsclient.Client
}

// NewNATSClientPublisher creates a new NATS client publisher
func NewNATSClientPublisher(client *natsclient.Client) *NATSClientPublisher {
	return &NATSClientPublisher{client: client}
}

// Publish publishes a message to the given subject
func (p *NATSClientPublisher) Publish(subject string, data []byte) error {
	return p.client.Publish(subject, data)
}

// ProducerService handles message production
type ProducerService struct {
	publisher MessagePublisher
	logger    *slog.Logger
}

// NewProducerService creates a new producer service
func NewProducerService(publisher MessagePublisher, logger *slog.Logger) *ProducerService {
	return &ProducerService{
		publisher: publisher,
		logger:    logger,
	}
}

// MessageData represents the message structure
type MessageData struct {
	Message string `json:"message"`
}

// StartupMessage represents the startup message structure
type StartupMessage struct {
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// PublishMessage publishes a message to the configured subject
func (s *ProducerService) PublishMessage(ctx context.Context, message string, subject string) error {
	if s.publisher == nil {
		return fmt.Errorf("publisher not initialized")
	}

	messageData := MessageData{
		Message: message,
	}

	data, err := json.Marshal(messageData)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to marshal message", "error", err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = s.publisher.Publish(subject, data)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to publish message", "error", err, "subject", subject)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	s.logger.InfoContext(ctx, "message published successfully", "subject", subject, "message", message)
	return nil
}

// PublishStartupMessage publishes a startup message
func (s *ProducerService) PublishStartupMessage(ctx context.Context, serviceName, version, subject string) error {
	if s.publisher == nil {
		return fmt.Errorf("publisher not initialized")
	}

	startupMessage := StartupMessage{
		Service:   serviceName,
		Version:   version,
		Timestamp: time.Now(),
		Status:    "started",
	}

	data, err := json.Marshal(startupMessage)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to marshal startup message", "error", err)
		return fmt.Errorf("failed to marshal startup message: %w", err)
	}

	err = s.publisher.Publish(subject, data)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to publish startup message", "error", err, "subject", subject)
		return fmt.Errorf("failed to publish startup message: %w", err)
	}

	s.logger.InfoContext(ctx, "startup message published successfully", "subject", subject, "service", serviceName, "version", version)
	return nil
}
