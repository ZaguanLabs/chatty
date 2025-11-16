package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

const (
	defaultTimeout   = 30 * time.Second
	streamingTimeout = 120 * time.Second
	cacheSize        = 128
)

// Message represents a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client handles HTTP communication with OpenAI-compatible APIs.
type Client struct {
	apiKey          string
	baseURL         string
	http            *http.Client
	streamBuf       *bufio.Writer
	bufMutex        sync.Mutex
	flushThreshold  int // Threshold in bytes before flushing buffer
	cache           *lru.Cache[string, string]
}

// NewClient creates a new API client.
func NewClient(apiKey, baseURL string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("api key cannot be empty")
	}
	if baseURL == "" {
		return nil, errors.New("base URL cannot be empty")
	}

	cache, err := lru.New[string, string](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &Client{
		apiKey:         apiKey,
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		http: &http.Client{
			Timeout: defaultTimeout,
		},
		flushThreshold: 256, // Set a reasonable default buffer size
		cache:          cache,
	}, nil
}

// Chat sends a chat completion request and returns the assistant's response.
func (c *Client) Chat(ctx context.Context, messages []Message, model string, temperature float64) (string, error) {
	if c == nil {
		return "", errors.New("client is nil")
	}

	// Generate a cache key for the request
	cacheKey, err := c.generateCacheKey(messages, model, temperature)
	if err != nil {
		// Log the error but proceed without caching
		fmt.Printf("Error generating cache key: %v\n", err)
	}

	// Check cache first
	if c.cache != nil && cacheKey != "" {
		if cached, ok := c.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}

	// Include temperature only if not an o3 model
	if !strings.HasPrefix(model, "o3") {
		reqBody["temperature"] = temperature
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", c.decodeError(bytes.NewReader(bodyBytes), resp.StatusCode)
	}

	response, err := c.decodeSuccess(resp.Body)
	if err != nil {
		return "", err
	}

	// Add to cache
	if c.cache != nil && cacheKey != "" {
		c.cache.Add(cacheKey, response)
	}

	return response, nil
}

// generateCacheKey creates a unique hash for a given set of messages and parameters.
func (c *Client) generateCacheKey(messages []Message, model string, temperature float64) (string, error) {
	// Create a struct to hold all cacheable data
	cacheable := struct {
		Messages    []Message `json:"messages"`
		Model       string    `json:"model"`
		Temperature float64   `json:"temperature"`
	}{
		Messages:    messages,
		Model:       model,
		Temperature: temperature,
	}

	// Marshal the data to JSON
	data, err := json.Marshal(cacheable)
	if err != nil {
		return "", fmt.Errorf("marshal cache key: %w", err)
	}

	// Hash the JSON data
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}


// ChatStream sends a streaming chat completion request and calls onChunk for each content delta.
func (c *Client) ChatStream(ctx context.Context, messages []Message, model string, temperature float64, onChunk func(string) error) error {
	if c == nil {
		return errors.New("client is nil")
	}

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}

	// Include temperature only if not an o3 model
	if !strings.HasPrefix(model, "o3") {
		reqBody["temperature"] = temperature
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	// Use a client with longer timeout for streaming
	ctx, cancel := context.WithTimeout(ctx, streamingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.decodeError(bytes.NewReader(bodyBytes), resp.StatusCode)
	}

	return c.processStream(resp.Body, onChunk)
}

func (c *Client) processStream(r io.Reader, onChunk func(string) error) error {
	var outputBuffer strings.Builder

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024), 64*1024) // Set max token size to 64KB

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Flush any remaining buffered content
			if outputBuffer.Len() > 0 {
				if err := onChunk(outputBuffer.String()); err != nil {
					return err
				}
			}
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			outputBuffer.WriteString(content)

			// Flush when buffer reaches threshold
			if outputBuffer.Len() >= c.flushThreshold {
				if err := onChunk(outputBuffer.String()); err != nil {
					return err
				}
				outputBuffer.Reset()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	// Flush any remaining content
	if outputBuffer.Len() > 0 {
		return onChunk(outputBuffer.String())
	}

	return nil
}

func (c *Client) decodeSuccess(r io.Reader) (string, error) {
	var response struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(r).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}

func (c *Client) decodeError(r io.Reader, status int) error {
	var apiErr struct {
		Error interface{} `json:"error"`
	}

	if err := json.NewDecoder(r).Decode(&apiErr); err != nil {
		return fmt.Errorf("api error (status %d): failed to decode body: %w", status, err)
	}

	var message string
	switch e := apiErr.Error.(type) {
	case string:
		message = e
	case map[string]interface{}:
		if msg, ok := e["message"].(string); ok {
			message = msg
		}
	}

	if message != "" {
		return fmt.Errorf("api error (status %d): %s", status, message)
	}

	return fmt.Errorf("api error (status %d)", status)
}
