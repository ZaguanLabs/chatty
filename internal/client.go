package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ZaguanLabs/chatty/internal/security"
	chattyErrors "github.com/ZaguanLabs/chatty/internal/errors"
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
	rateLimiter     *security.RateLimiter
	apiTokenBucket  *security.APITokenBucket
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
		return "", chattyErrors.NewSecureValidationError("Invalid client", "Client is nil", "client", nil)
	}

	// Check rate limiting
	if c.rateLimiter != nil {
		if !c.rateLimiter.Allow(c.apiKey) {
			remainingTime := c.rateLimiter.GetRemainingTime(c.apiKey)
			return "", chattyErrors.NewSecureNetworkError(
				"Rate limit exceeded",
				fmt.Sprintf("Rate limit exceeded, please try again in %v", remainingTime),
				c.baseURL,
				429,
				nil,
			)
		}
	}

	// Check token bucket
	if c.apiTokenBucket != nil {
		if !c.apiTokenBucket.Allow() {
			return "", chattyErrors.NewSecureNetworkError(
				"API temporarily unavailable",
				"API token bucket exhausted",
				c.baseURL,
				503,
				nil,
			)
		}
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

	// Set security headers
	setSecurityHeaders(req)

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
		return chattyErrors.NewSecureValidationError("Invalid client", "Client is nil", "client", nil)
	}

	// Check rate limiting
	if c.rateLimiter != nil {
		if !c.rateLimiter.Allow(c.apiKey) {
			remainingTime := c.rateLimiter.GetRemainingTime(c.apiKey)
			return chattyErrors.NewSecureNetworkError(
				"Rate limit exceeded",
				fmt.Sprintf("Rate limit exceeded, please try again in %v", remainingTime),
				c.baseURL,
				429,
				nil,
			)
		}
	}

	// Check token bucket
	if c.apiTokenBucket != nil {
		if !c.apiTokenBucket.Allow() {
			return errors.New("API token bucket exhausted, please try again later")
		}
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

	// Set security headers
	setSecurityHeaders(req)

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

// NewSecureClient creates a new secure API client with enhanced security features
func NewSecureClient(apiKey, baseURL string) (*Client, error) {
	// Validate inputs
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("API key cannot be empty")
	}

	// Validate API key format
	if len(apiKey) < 10 || len(apiKey) > 500 {
		return nil, errors.New("API key length is invalid")
	}

	if strings.Contains(apiKey, " ") {
		return nil, errors.New("API key contains spaces")
	}

	if strings.Contains(apiKey, "${") {
		return nil, errors.New("API key contains template variables")
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("base URL cannot be empty")
	}

	// Validate URL format
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, errors.New("base URL must start with http:// or https://")
	}

	// Create cache
	cache, err := lru.New[string, string](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	// Create rate limiter - 10 requests per minute per API key
	rateLimitConfig := security.RateLimitConfig{
		MaxRequests:     10,
		WindowSize:      time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
	rateLimiter := security.NewRateLimiter(rateLimitConfig)

	// Create API token bucket - 100 tokens max, refill 1 token per second
	tokenBucket := security.NewAPITokenBucket(100, 1)

	// Create secure HTTP client
	transport := createSecureHTTPTransport()
	httpClient := &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}

	client := &Client{
		apiKey:         apiKey,
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		http:           httpClient,
		flushThreshold: 256,
		cache:          cache,
		rateLimiter:    rateLimiter,
		apiTokenBucket: tokenBucket,
	}

	// Securely clear the API key from the parameter
	secureClear(apiKey)

	return client, nil
}

// secureClear securely clears sensitive string data from memory
func secureClear(s string) {
	// Convert string to byte slice and overwrite
	b := []byte(s)
	for i := range b {
		b[i] = 0
	}
	// Force garbage collection to help clear memory
	runtime.GC()
}

// secureClearBytes securely clears sensitive byte data from memory
func secureClearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// generateSecureNonce generates a cryptographically secure nonce
func generateSecureNonce() (string, error) {
	// Generate 16 random bytes
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure nonce: %w", err)
	}

	// Convert to hex string
	nonce := hex.EncodeToString(bytes)

	// Clear the bytes from memory
	secureClearBytes(bytes)

	return nonce, nil
}

// GetRateLimitStats returns rate limiting statistics for the client
func (c *Client) GetRateLimitStats() (requests int, remainingTime time.Duration, allowed bool) {
	if c.rateLimiter == nil {
		return 0, 0, true
	}
	return c.rateLimiter.GetStats(c.apiKey)
}

// GetTokenBucketTokens returns the current number of tokens in the bucket
func (c *Client) GetTokenBucketTokens() int {
	if c.apiTokenBucket == nil {
		return 0
	}
	return c.apiTokenBucket.GetTokens()
}

// ResetRateLimiter resets the rate limiter for this client
func (c *Client) ResetRateLimiter() {
	if c.rateLimiter != nil {
		c.rateLimiter.Reset(c.apiKey)
	}
}

// setSecurityHeaders adds security headers to HTTP requests
func setSecurityHeaders(req *http.Request) {
	if req == nil {
		return
	}

	// Generate a secure nonce for this request
	nonce, err := generateSecureNonce()
	if err == nil {
		req.Header.Set("X-Request-Nonce", nonce)
	}

	// Security headers
	req.Header.Set("X-Content-Type-Options", "nosniff")
	req.Header.Set("X-Frame-Options", "DENY")
	req.Header.Set("X-XSS-Protection", "1; mode=block")
	req.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Remove potentially sensitive headers
	req.Header.Del("User-Agent") // Remove or set to generic value
	req.Header.Set("User-Agent", "Chatty/1.0")
}
func createSecureHTTPTransport() *http.Transport {
	// Create a certificate pool with system roots
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		// Fallback to empty pool if system cert pool is not available
		rootCAs = x509.NewCertPool()
	}

	// Create secure TLS configuration
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Require TLS 1.2 or higher
		MaxVersion: tls.VersionTLS13, // Support up to TLS 1.3
		RootCAs:    rootCAs,
		// Security features
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		// Prevent common attacks
		InsecureSkipVerify: false, // Always verify certificates
		Renegotiation:      tls.RenegotiateNever,
	}

	return &http.Transport{
		TLSClientConfig: tlsConfig,
		// Additional security settings
		DisableKeepAlives:  false, // Enable keep-alives for performance
		DisableCompression: false, // Enable compression
		MaxIdleConns:       10,    // Limit idle connections
		IdleConnTimeout:    90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
