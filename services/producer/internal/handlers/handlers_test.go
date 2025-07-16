package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MockProducerService is a mock implementation of ProducerService for testing
type MockProducerService struct {
	mock.Mock
}

func (m *MockProducerService) PublishMessage(ctx context.Context, message string, subject string) error {
	args := m.Called(ctx, message, subject)
	return args.Error(0)
}

func (m *MockProducerService) PublishStartupMessage(ctx context.Context, serviceName, version, subject string) error {
	args := m.Called(ctx, serviceName, version, subject)
	return args.Error(0)
}

// HandlerTestSuite defines a test suite for handler tests
type HandlerTestSuite struct {
	suite.Suite
	handler     *Handler
	mockService *MockProducerService
	logger      *slog.Logger
}

// SetupTest runs before each test
func (suite *HandlerTestSuite) SetupTest() {
	suite.logger = slog.Default()
	suite.mockService = new(MockProducerService)
	suite.handler = NewHandler(suite.logger)
	suite.handler.SetProducerService(suite.mockService, "test.subject")
}

// TearDownTest runs after each test
func (suite *HandlerTestSuite) TearDownTest() {
	suite.mockService.AssertExpectations(suite.T())
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) TestHandler_Hello() {
	version := "1.0.0"

	tests := []struct {
		name           string
		method         string
		url            string
		headers        map[string]string
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "successful GET request",
			method:         http.MethodGet,
			url:            "/",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Equal(t, "producer", w.Header().Get("X-Service"))

				var response HelloResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "Hello, World!", response.Message)
				assert.Equal(t, version, response.Version)
				assert.WithinDuration(t, time.Now(), response.Timestamp, 5*time.Second)
			},
		},
		{
			name:           "successful POST request",
			method:         http.MethodPost,
			url:            "/",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response HelloResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "Hello, World!", response.Message)
			},
		},
		{
			name:           "with query parameters",
			method:         http.MethodGet,
			url:            "/?param=value&test=123",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response HelloResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "Hello, World!", response.Message)
			},
		},
		{
			name:   "with custom headers",
			method: http.MethodGet,
			url:    "/",
			headers: map[string]string{
				"User-Agent":    "test-agent/2.0",
				"X-Request-ID":  "req-456",
				"Authorization": "Bearer token123",
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req, err := http.NewRequest(tt.method, tt.url, nil)
			require.NoError(suite.T(), err)

			// Add custom headers if provided
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			helloHandler := suite.handler.Hello(version)
			helloHandler.ServeHTTP(w, req)

			assert.Equal(suite.T(), tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(suite.T(), w)
			}
		})
	}
}

func (suite *HandlerTestSuite) TestHandler_CreateMessage() {
	tests := []struct {
		name           string
		method         string
		body           interface{}
		setupMock      func(*MockProducerService)
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "successful message creation",
			method: http.MethodPost,
			body: CreateMessageRequest{
				Message: "test message",
			},
			setupMock: func(m *MockProducerService) {
				m.On("PublishMessage", mock.Anything, "test message", "test.subject").Return(nil)
			},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response CreateMessageResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "true", response.Success)
			},
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			body:           nil,
			setupMock:      func(m *MockProducerService) {},
			expectedStatus: http.StatusMethodNotAllowed,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "method not allowed", response.Error)
				assert.WithinDuration(t, time.Now(), response.Timestamp, 5*time.Second)
			},
		},
		{
			name:           "invalid JSON body",
			method:         http.MethodPost,
			body:           "invalid json",
			setupMock:      func(m *MockProducerService) {},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "invalid request body", response.Error)
			},
		},
		{
			name:   "empty message",
			method: http.MethodPost,
			body: CreateMessageRequest{
				Message: "",
			},
			setupMock:      func(m *MockProducerService) {},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "message cannot be empty", response.Error)
			},
		},
		{
			name:   "publish message fails",
			method: http.MethodPost,
			body: CreateMessageRequest{
				Message: "test message",
			},
			setupMock: func(m *MockProducerService) {
				m.On("PublishMessage", mock.Anything, "test message", "test.subject").
					Return(errors.New("publish failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "failed to publish message", response.Error)
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Setup mock expectations
			tt.setupMock(suite.mockService)

			var bodyReader *bytes.Reader
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					bodyReader = bytes.NewReader([]byte(str))
				} else {
					bodyBytes, err := json.Marshal(tt.body)
					require.NoError(suite.T(), err)
					bodyReader = bytes.NewReader(bodyBytes)
				}
			} else {
				bodyReader = bytes.NewReader([]byte{})
			}

			req, err := http.NewRequest(tt.method, "/message/create", bodyReader)
			require.NoError(suite.T(), err)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			suite.handler.CreateMessage(w, req)

			assert.Equal(suite.T(), tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(suite.T(), w)
			}
		})
	}
}

func (suite *HandlerTestSuite) TestHandler_CreateMessage_NoProducerService() {
	// Test when producer service is not set
	handler := NewHandler(suite.logger)
	// Don't set producer service

	body := CreateMessageRequest{Message: "test"}
	bodyBytes, err := json.Marshal(body)
	require.NoError(suite.T(), err)

	req, err := http.NewRequest(http.MethodPost, "/message/create", bytes.NewReader(bodyBytes))
	require.NoError(suite.T(), err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateMessage(w, req)

	assert.Equal(suite.T(), http.StatusServiceUnavailable, w.Code)

	var response ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "message publishing not available", response.Error)
}

func (suite *HandlerTestSuite) TestHandler_Health() {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "successful health check",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "ok", response["status"])
				assert.Equal(t, "producer", response["service"])
				assert.Contains(t, response, "timestamp")
			},
		},
		{
			name:           "POST to health endpoint",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "ok", response["status"])
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req, err := http.NewRequest(tt.method, "/health", nil)
			require.NoError(suite.T(), err)

			w := httptest.NewRecorder()
			healthHandler := suite.handler.Health()
			healthHandler.ServeHTTP(w, req)

			assert.Equal(suite.T(), tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(suite.T(), w)
			}
		})
	}
}

func (suite *HandlerTestSuite) TestHandler_writeErrorResponse() {
	tests := []struct {
		name           string
		message        string
		statusCode     int
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "internal server error",
			message:        "internal server error",
			statusCode:     http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "internal server error", response.Error)
				assert.WithinDuration(t, time.Now(), response.Timestamp, 5*time.Second)
			},
		},
		{
			name:           "bad request",
			message:        "bad request",
			statusCode:     http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "bad request", response.Error)
			},
		},
		{
			name:           "custom error message",
			message:        "custom validation failed",
			statusCode:     http.StatusUnprocessableEntity,
			expectedStatus: http.StatusUnprocessableEntity,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "custom validation failed", response.Error)
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			suite.handler.writeErrorResponse(w, tt.message, tt.statusCode)

			assert.Equal(suite.T(), tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(suite.T(), w)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	logger := slog.Default()
	handler := NewHandler(logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
	assert.Nil(t, handler.producerService)
	assert.Empty(t, handler.messageSubject)
}

func TestHandler_SetProducerService(t *testing.T) {
	logger := slog.Default()
	handler := NewHandler(logger)
	mockService := new(MockProducerService)
	subject := "test.subject"

	handler.SetProducerService(mockService, subject)

	assert.Equal(t, mockService, handler.producerService)
	assert.Equal(t, subject, handler.messageSubject)
}

// Test JSON marshaling/unmarshaling of response structures
func TestResponseStructures(t *testing.T) {
	t.Run("HelloResponse", func(t *testing.T) {
		original := HelloResponse{
			Message:   "Hello, World!",
			Timestamp: time.Now(),
			Version:   "1.0.0",
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled HelloResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, original.Message, unmarshaled.Message)
		assert.Equal(t, original.Version, unmarshaled.Version)
		assert.WithinDuration(t, original.Timestamp, unmarshaled.Timestamp, time.Second)
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		original := ErrorResponse{
			Error:     "test error",
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled ErrorResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, original.Error, unmarshaled.Error)
		assert.WithinDuration(t, original.Timestamp, unmarshaled.Timestamp, time.Second)
	})

	t.Run("CreateMessageRequest", func(t *testing.T) {
		original := CreateMessageRequest{
			Message: "test message",
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled CreateMessageRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, original.Message, unmarshaled.Message)
	})
}

// Benchmark tests
func BenchmarkHandler_Hello(b *testing.B) {
	logger := slog.Default()
	handler := NewHandler(logger)
	version := "1.0.0"
	helloHandler := handler.Hello(version)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "benchmark-test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		helloHandler.ServeHTTP(w, req)
	}
}

func BenchmarkHandler_Health(b *testing.B) {
	logger := slog.Default()
	handler := NewHandler(logger)
	healthHandler := handler.Health()

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		healthHandler.ServeHTTP(w, req)
	}
}

func BenchmarkHandler_CreateMessage(b *testing.B) {
	logger := slog.Default()
	mockService := new(MockProducerService)
	handler := NewHandler(logger)
	handler.SetProducerService(mockService, "test.subject")

	// Setup mock to always succeed
	mockService.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	body := CreateMessageRequest{Message: "benchmark test"}
	bodyBytes, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/message/create", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.CreateMessage(w, req)
	}
}

// Table-driven tests for edge cases
func TestHandler_EdgeCases(t *testing.T) {
	logger := slog.Default()
	handler := NewHandler(logger)

	t.Run("Hello with empty version", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		helloHandler := handler.Hello("")
		helloHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HelloResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Empty(t, response.Version)
	})

	t.Run("CreateMessage with very long message", func(t *testing.T) {
		mockService := new(MockProducerService)
		handler.SetProducerService(mockService, "test.subject")

		longMessage := strings.Repeat("a", 10000)
		mockService.On("PublishMessage", mock.Anything, longMessage, "test.subject").Return(nil)

		body := CreateMessageRequest{Message: longMessage}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/message/create", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.CreateMessage(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("CreateMessage with special characters", func(t *testing.T) {
		mockService := new(MockProducerService)
		handler.SetProducerService(mockService, "test.subject")

		specialMessage := "Hello! 🚀 Special chars: äöü ñ 中文 العربية"
		mockService.On("PublishMessage", mock.Anything, specialMessage, "test.subject").Return(nil)

		body := CreateMessageRequest{Message: specialMessage}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/message/create", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.CreateMessage(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})
}
