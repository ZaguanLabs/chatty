package mocks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ZaguanLabs/chatty/internal/storage"
)

// MockAPI simulates an OpenAI-compatible API for testing
type MockAPI struct {
	baseURL       string
	apiKey        string
	responses     []string
	responseIndex int
	mu            sync.Mutex
	callCount     int
	delay         time.Duration
}

// NewMockAPI creates a new mock API client
func NewMockAPI(baseURL, apiKey string) *MockAPI {
	return &MockAPI{
		baseURL:       baseURL,
		apiKey:        apiKey,
		responses:     []string{"Hello! How can I help you today?"},
		responseIndex: 0,
		delay:         0,
	}
}

// SetResponse sets a custom response
func (m *MockAPI) SetResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = []string{response}
	m.responseIndex = 0
}

// SetResponses sets multiple responses that will be returned in sequence
func (m *MockAPI) SetResponses(responses []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = responses
	m.responseIndex = 0
}

// SetDelay sets a simulated network delay
func (m *MockAPI) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

// Chat simulates a non-streaming chat request
func (m *MockAPI) Chat(ctx context.Context, history []storage.Message, model string, temperature float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check context
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	
	// Validate input
	if m.apiKey == "" {
		return "", fmt.Errorf("unauthorized: missing API key")
	}
	
	if model == "" {
		return "", fmt.Errorf("bad request: missing model")
	}
	
	// Get response
	if len(m.responses) == 0 {
		return "No response configured", nil
	}
	
	response := m.responses[m.responseIndex%len(m.responses)]
	m.responseIndex++
	
	return response, nil
}

// ChatStream simulates a streaming chat request
func (m *MockAPI) ChatStream(ctx context.Context, history []storage.Message, model string, temperature float64, callback func(chunk string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Validate input
	if m.apiKey == "" {
		return fmt.Errorf("unauthorized: missing API key")
	}
	
	if model == "" {
		return fmt.Errorf("bad request: missing model")
	}
	
	if callback == nil {
		return fmt.Errorf("bad request: missing callback")
	}
	
	// Get response
	if len(m.responses) == 0 {
		return callback("No response configured")
	}
	
	response := m.responses[m.responseIndex%len(m.responses)]
	m.responseIndex++
	
	// Stream response in chunks
	words := strings.Fields(response)
	for _, word := range words {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Simulate delay
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		// Send chunk
		if err := callback(word + " "); err != nil {
			return err
		}
	}
	
	return nil
}

// GetCallCount returns the number of API calls made
func (m *MockAPI) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ResetCallCount resets the call counter
func (m *MockAPI) ResetCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
}

// MockStorage simulates a storage backend for testing
type MockStorage struct {
	mu         sync.RWMutex
	sessions   map[int64]*storage.SessionSummary
	messages   map[int64][]storage.Message
	nextID     int64
	errors     map[string]error
	delay      time.Duration
	callCount  int
}

// NewMockStorage creates a new mock storage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		sessions: make(map[int64]*storage.SessionSummary),
		messages: make(map[int64][]storage.Message),
		errors:   make(map[string]error),
		nextID:   1,
	}
}

// SetError simulates an error for a specific operation
func (m *MockStorage) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[operation] = err
}

// SetDelay sets a simulated database delay
func (m *MockStorage) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

// CreateSession implements storage.Store interface
func (m *MockStorage) CreateSession(ctx context.Context, name string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check for simulated error
	if err := m.errors["CreateSession"]; err != nil {
		return 0, err
	}
	
	// Check context
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	
	// Create session
	session := &storage.SessionSummary{
		ID:        m.nextID,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	m.sessions[m.nextID] = session
	m.nextID++
	
	return session.ID, nil
}

// AppendMessage implements storage.Store interface
func (m *MockStorage) AppendMessage(ctx context.Context, sessionID int64, message storage.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check for simulated error
	if err := m.errors["AppendMessage"]; err != nil {
		return err
	}
	
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Store message
	m.messages[sessionID] = append(m.messages[sessionID], message)
	
	return nil
}

// AppendMessagesBatch implements batch operations
func (m *MockStorage) AppendMessagesBatch(ctx context.Context, sessionID int64, messages []storage.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check for simulated error
	if err := m.errors["AppendMessagesBatch"]; err != nil {
		return err
	}
	
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Store all messages
	for _, msg := range messages {
		m.messages[sessionID] = append(m.messages[sessionID], msg)
	}
	
	return nil
}

// ListSessions implements storage.Store interface
func (m *MockStorage) ListSessions(ctx context.Context, limit int) ([]storage.SessionSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check for simulated error
	if err := m.errors["ListSessions"]; err != nil {
		return nil, err
	}
	
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	// Get sessions
	sessions := make([]storage.SessionSummary, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, *session)
	}
	
	// Sort by updated time (most recent first)
	if len(sessions) > 1 {
		for i := 0; i < len(sessions); i++ {
			for j := i + 1; j < len(sessions); j++ {
				if sessions[i].UpdatedAt.Before(sessions[j].UpdatedAt) {
					sessions[i], sessions[j] = sessions[j], sessions[i]
				}
			}
		}
	}
	
	// Apply limit
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	
	return sessions, nil
}

// LoadSession implements storage.Store interface
func (m *MockStorage) LoadSession(ctx context.Context, id int64) (*storage.Transcript, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount++
	
	// Check for simulated error
	if err := m.errors["LoadSession"]; err != nil {
		return nil, err
	}
	
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Simulate delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	// Get session
	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %d not found", id)
	}
	
	// Get messages
	messages, ok := m.messages[id]
	if !ok {
		messages = []storage.Message{}
	}
	
	return &storage.Transcript{
		Summary:  *session,
		Messages: messages,
	}, nil
}

// Close implements storage.Store interface (no-op for mock)
func (m *MockStorage) Close() error {
	return nil
}

// GetCallCount returns the number of storage operations performed
func (m *MockStorage) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ResetCallCount resets the call counter
func (m *MockStorage) ResetCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
}

// ClearAll clears all stored data
func (m *MockStorage) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[int64]*storage.SessionSummary)
	m.messages = make(map[int64][]storage.Message)
	m.nextID = 1
	m.errors = make(map[string]error)
}