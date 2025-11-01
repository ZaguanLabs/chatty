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

const defaultTimeout = 30 * time.Second

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
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// StreamCallback is called for each token received during streaming.
type StreamCallback func(token string) error

// Chat sends a chat completion request and returns the assistant's response.
func (c *Client) Chat(ctx context.Context, messages []Message, model string, temperature float64) (string, error) {
	return c.chat(ctx, messages, model, temperature, false, nil)
}

// ChatStream sends a streaming chat completion request and calls the callback for each token.
func (c *Client) ChatStream(ctx context.Context, messages []Message, model string, temperature float64, callback StreamCallback) (string, error) {
	return c.chat(ctx, messages, model, temperature, true, callback)
}

func (c *Client) chat(ctx context.Context, messages []Message, model string, temperature float64, stream bool, callback StreamCallback) (string, error) {
	if c == nil {
		return "", errors.New("client is nil")
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"stream":      stream,
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
		return "", c.decodeError(resp.Body, resp.StatusCode)
	}

	if stream {
		return c.decodeStream(resp.Body, callback)
	}

	return c.decodeSuccess(resp.Body)
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

func (c *Client) decodeStream(r io.Reader, callback StreamCallback) (string, error) {
	scanner := bufio.NewScanner(r)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			break
		}

		// Parse the JSON chunk
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			token := chunk.Choices[0].Delta.Content
			fullContent.WriteString(token)

			// Call the callback with the token
			if callback != nil {
				if err := callback(token); err != nil {
					return fullContent.String(), err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullContent.String(), fmt.Errorf("stream error: %w", err)
	}

	return fullContent.String(), nil
}

func (c *Client) decodeError(r io.Reader, status int) error {
	// Read the raw body first
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("api error (status %d): failed to read response body: %w", status, err)
	}

	// Try to decode as JSON error
	var apiErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If JSON decode fails, return the raw body (truncated if too long)
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return fmt.Errorf("api error (status %d): %s", status, bodyStr)
	}

	if apiErr.Error.Message != "" {
		return fmt.Errorf("api error (status %d): %s", status, apiErr.Error.Message)
	}

	return fmt.Errorf("api error (status %d)", status)
}
