package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	streamingTimeout  = 120 * time.Second
)

// Message represents a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client handles HTTP communication with OpenAI-compatible APIs.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
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

	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// Chat sends a chat completion request and returns the assistant's response.
func (c *Client) Chat(ctx context.Context, messages []Message, model string, temperature float64) (string, error) {
	if c == nil {
		return "", errors.New("client is nil")
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

	return c.decodeSuccess(resp.Body)
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
	streamClient := &http.Client{
		Timeout: streamingTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := streamClient.Do(req)
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
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
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
			if err := onChunk(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
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
