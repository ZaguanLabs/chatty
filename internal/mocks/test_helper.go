package mocks

import (
	"context"
	"testing"
	"time"

	"github.com/ZaguanLabs/chatty/internal/storage"
)

// TestHelper provides utilities for testing with mocks
type TestHelper struct {
	api      *MockAPI
	storage  *MockStorage
	cleanup  func()
}

// NewTestHelper creates a new test helper with fresh mocks
func NewTestHelper() *TestHelper {
	api := NewMockAPI("http://mock-api.com", "test-key")
	storage := NewMockStorage()
	
	return &TestHelper{
		api:     api,
		storage: storage,
	}
}

// SetupTest configures the test helper with default values
func (h *TestHelper) SetupTest() {
	// Set realistic defaults
	h.api.SetDelay(10 * time.Millisecond)
	h.storage.SetDelay(5 * time.Millisecond)
	h.api.SetResponse("Test response from mock API")
}

// Teardown cleans up resources
func (h *TestHelper) Teardown() {
	if h.cleanup != nil {
		h.cleanup()
	}
}

// WithTimeout sets a custom timeout for operations
func (h *TestHelper) WithTimeout(timeout time.Duration) *TestHelper {
	return h
}

// WithError injects a specific error for testing error handling
func (h *TestHelper) WithError(operation string, err error) *TestHelper {
	h.storage.SetError(operation, err)
	return h
}

// WithNetworkError simulates network failures
func (h *TestHelper) WithNetworkError() *TestHelper {
	h.api.SetDelay(5 * time.Second) // Timeout
	return h
}

// WithSlowResponse simulates slow API responses
func (h *TestHelper) WithSlowResponse() *TestHelper {
	h.api.SetDelay(500 * time.Millisecond)
	return h
}

// GetAPI returns the mock API instance
func (h *TestHelper) GetAPI() *MockAPI {
	return h.api
}

// GetStorage returns the mock storage instance
func (h *TestHelper) GetStorage() *MockStorage {
	return h.storage
}

// Helper functions for common test scenarios

// SuccessScenario creates a test helper configured for successful operations
func SuccessScenario() *TestHelper {
	h := NewTestHelper()
	h.api.SetResponses([]string{
		"Hello! How can I help you today?",
		"That's a great question!",
		"Let me think about that...",
		"I understand your concern.",
	})
	return h
}

// ErrorScenario creates a test helper configured for error handling
func ErrorScenario() *TestHelper {
	h := NewTestHelper()
	h.api.SetResponse("")
	h.api.SetDelay(10 * time.Millisecond)
	return h
}

// TimeoutScenario creates a test helper configured for timeout testing
func TimeoutScenario() *TestHelper {
	h := NewTestHelper()
	h.api.SetDelay(10 * time.Second) // Will timeout
	return h
}

// PerformanceScenario creates a test helper for performance testing
func PerformanceScenario() *TestHelper {
	h := NewTestHelper()
	h.api.SetDelay(1 * time.Millisecond) // Very fast for benchmarking
	h.storage.SetDelay(0) // Instant storage
	return h
}

// BenchmarkContext creates a context suitable for benchmarking
func BenchmarkContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// Helper to assert operations completed successfully
func AssertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// Helper to assert specific errors
func AssertError(t testing.TB, expectedErr error, actualErr error) {
	t.Helper()
	if actualErr == nil {
		t.Fatalf("Expected error %v, got nil", expectedErr)
	}
	if expectedErr != nil && actualErr.Error() != expectedErr.Error() {
		t.Fatalf("Expected error %v, got %v", expectedErr, actualErr)
	}
}

// Helper to verify storage operations
func AssertStorageCallCount(t testing.TB, storage *MockStorage, expected int) {
	t.Helper()
	actual := storage.GetCallCount()
	if actual != expected {
		t.Fatalf("Expected %d storage calls, got %d", expected, actual)
	}
}

// Helper to verify API calls
func AssertAPICallCount(t testing.TB, api *MockAPI, expected int) {
	t.Helper()
	actual := api.GetCallCount()
	if actual != expected {
		t.Fatalf("Expected %d API calls, got %d", expected, actual)
	}
}

// Helper to verify message content
func AssertMessage(t testing.TB, expected, actual storage.Message) {
	t.Helper()
	if expected.Role != actual.Role {
		t.Fatalf("Expected role %s, got %s", expected.Role, actual.Role)
	}
	if expected.Content != actual.Content {
		t.Fatalf("Expected content %s, got %s", expected.Content, actual.Content)
	}
}

// Helper to wait for async operations
func WaitForCompletion(timeout time.Duration, fn func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

// CreateTestMessages creates a slice of test messages
func CreateTestMessages(count int) []storage.Message {
	messages := make([]storage.Message, count)
	for i := 0; i < count; i++ {
		messages[i] = storage.Message{
			Role:      []string{"user", "assistant"}[i%2],
			Content:   "Test message " + string(rune('A'+i)),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
	}
	return messages
}

// CreateTestSessions creates test session summaries
func CreateTestSessions(count int) []storage.SessionSummary {
	sessions := make([]storage.SessionSummary, count)
	now := time.Now()
	for i := 0; i < count; i++ {
		sessions[i] = storage.SessionSummary{
			ID:           int64(i + 1),
			Name:         "Test Session " + string(rune('A'+i)),
			CreatedAt:    now.Add(time.Duration(i) * time.Hour),
			UpdatedAt:    now.Add(time.Duration(i+1) * time.Hour),
			MessageCount: (i + 1) * 2,
		}
	}
	return sessions
}