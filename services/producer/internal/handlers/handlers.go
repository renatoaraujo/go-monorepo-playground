package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/renatoaraujo/go-monorepo-playground/services/producer/internal/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	logger          *slog.Logger
	producerService service.Producer // use interface for easier testing
	messageSubject  string
}

// NewHandler creates a new handler with dependencies
func NewHandler(logger *slog.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

// SetProducerService sets the producer service for the handler
func (h *Handler) SetProducerService(producerService service.Producer, messageSubject string) {
	h.producerService = producerService
	h.messageSubject = messageSubject
}

// HelloResponse represents the response structure
type HelloResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}

// CreateMessageRequest represents the request body for creating a message
type CreateMessageRequest struct {
	Message string `json:"message"`
}

// CreateMessageResponse represents the response for creating a message
type CreateMessageResponse struct {
	Success string `json:"success"`
}

// ErrorResponse represents an error response structure
type ErrorResponse struct {
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// Hello handles the hello endpoint
func (h *Handler) Hello(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Add tracing information
		span := trace.SpanFromContext(ctx)
		bag := baggage.FromContext(ctx)

		// Add span attributes
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("user_agent", r.UserAgent()),
		)

		// Handle baggage if present
		if username := bag.Member("username"); username.Value() != "" {
			span.AddEvent("handling request", trace.WithAttributes(
				attribute.String("username", username.Value()),
			))
		}

		// Create response
		response := HelloResponse{
			Message:   "Hello, World!",
			Timestamp: time.Now(),
			Version:   version,
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service", "producer")

		// Encode and send response
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.ErrorContext(ctx, "failed to encode response", "error", err)
			h.writeErrorResponse(w, "internal server error", http.StatusInternalServerError)
			return
		}

		h.logger.InfoContext(ctx, "hello request processed successfully",
			"method", r.Method,
			"url", r.URL.String(),
			"user_agent", r.UserAgent(),
		)
	}
}

// CreateMessage handles POST /message/create endpoint
func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow POST method
	if r.Method != http.MethodPost {
		h.logger.WarnContext(ctx, "method not allowed", "method", r.Method)
		h.writeErrorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req CreateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "failed to decode request body", "error", err)
		h.writeErrorResponse(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate message
	if req.Message == "" {
		h.logger.WarnContext(ctx, "empty message in request")
		h.writeErrorResponse(w, "message cannot be empty", http.StatusBadRequest)
		return
	}

	// Check if producer service is available
	if h.producerService == nil {
		h.logger.ErrorContext(ctx, "producer service not available")
		h.writeErrorResponse(w, "message publishing not available", http.StatusServiceUnavailable)
		return
	}

	// Publish message
	if err := h.producerService.PublishMessage(ctx, req.Message, h.messageSubject); err != nil {
		h.logger.ErrorContext(ctx, "failed to publish message", "error", err)
		h.writeErrorResponse(w, "failed to publish message", http.StatusInternalServerError)
		return
	}

	// Return success response
	h.logger.InfoContext(ctx, "message created successfully", "message", req.Message)
	response := CreateMessageResponse{Success: "true"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Health handles the health check endpoint
func (h *Handler) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now(),
			"service":   "producer",
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.ErrorContext(ctx, "failed to encode health response", "error", err)
			h.writeErrorResponse(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// writeErrorResponse writes an error response
func (h *Handler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ErrorResponse{
		Error:     message,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
