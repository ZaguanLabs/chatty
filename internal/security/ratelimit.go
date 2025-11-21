package security

import (
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	mu         sync.RWMutex
	requests   map[string][]time.Time
	maxRequests int
	windowSize time.Duration
	cleanupInterval time.Duration
}

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	MaxRequests     int           // Maximum number of requests allowed
	WindowSize      time.Duration // Time window for rate limiting
	CleanupInterval time.Duration // How often to clean up old entries
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxRequests:     100,         // 100 requests
		WindowSize:      time.Minute, // per minute
		CleanupInterval: 5 * time.Minute, // cleanup every 5 minutes
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		requests:        make(map[string][]time.Time),
		maxRequests:     config.MaxRequests,
		windowSize:      config.WindowSize,
		cleanupInterval: config.CleanupInterval,
	}
	
	// Start cleanup goroutine
	go rl.cleanupRoutine()
	
	return rl
}

// Allow checks if a request is allowed for the given key
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Get or create request history for this key
	requests, exists := rl.requests[key]
	if !exists {
		rl.requests[key] = []time.Time{now}
		return true
	}
	
	// Remove old requests outside the window
	validRequests := make([]time.Time, 0)
	cutoff := now.Add(-rl.windowSize)
	
	for _, timestamp := range requests {
		if timestamp.After(cutoff) {
			validRequests = append(validRequests, timestamp)
		}
	}
	
	// Check if we can add a new request
	if len(validRequests) < rl.maxRequests {
		validRequests = append(validRequests, now)
		rl.requests[key] = validRequests
		return true
	}
	
	// Rate limit exceeded
	rl.requests[key] = validRequests
	return false
}

// GetRemainingTime returns the time until the next request is allowed
func (rl *RateLimiter) GetRemainingTime(key string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	requests, exists := rl.requests[key]
	if !exists || len(requests) == 0 {
		return 0
	}
	
	// Find the oldest request in the current window
	cutoff := time.Now().Add(-rl.windowSize)
	oldestInWindow := time.Now()
	
	for _, timestamp := range requests {
		if timestamp.After(cutoff) && timestamp.Before(oldestInWindow) {
			oldestInWindow = timestamp
		}
	}
	
	// Calculate remaining time until this request is outside the window
	remainingTime := rl.windowSize - time.Since(oldestInWindow)
	if remainingTime < 0 {
		return 0
	}
	
	return remainingTime
}

// GetStats returns statistics for a given key
func (rl *RateLimiter) GetStats(key string) (int, time.Duration, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	requests, exists := rl.requests[key]
	if !exists {
		return 0, 0, true
	}
	
	// Count valid requests in the current window
	now := time.Now()
	cutoff := now.Add(-rl.windowSize)
	validCount := 0
	
	for _, timestamp := range requests {
		if timestamp.After(cutoff) {
			validCount++
		}
	}
	
	remainingTime := rl.GetRemainingTime(key)
	allowed := validCount < rl.maxRequests
	
	return validCount, remainingTime, allowed
}

// cleanupRoutine periodically cleans up old entries
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		rl.performCleanup()
	}
}

// performCleanup removes old entries
func (rl *RateLimiter) performCleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.cleanupInterval)
	
	for key, requests := range rl.requests {
		// Remove old requests
		validRequests := make([]time.Time, 0)
		for _, timestamp := range requests {
			if timestamp.After(cutoff) {
				validRequests = append(validRequests, timestamp)
			}
		}
		
		// If no valid requests remain, remove the key entirely
		if len(validRequests) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = validRequests
		}
	}
}

// Reset resets the rate limiter for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	delete(rl.requests, key)
}

// ResetAll resets all rate limiting data
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.requests = make(map[string][]time.Time)
}

// GetTotalKeys returns the total number of tracked keys
func (rl *RateLimiter) GetTotalKeys() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	return len(rl.requests)
}

// Stop stops the cleanup routine
func (rl *RateLimiter) Stop() {
	// This is a simple implementation - in a production system,
	// you might want to use a context for proper cleanup
	rl.ResetAll()
}

// APITokenBucket provides token bucket rate limiting for API calls
type APITokenBucket struct {
	tokens       int
	maxTokens    int
	refillRate   int
	lastRefill   time.Time
	mu           sync.Mutex
}

// NewAPITokenBucket creates a new token bucket for API rate limiting
func NewAPITokenBucket(maxTokens, refillRate int) *APITokenBucket {
	return &APITokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a token is available
func (tb *APITokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate
	
	if tokensToAdd > 0 {
		tb.tokens = min(tb.tokens+tokensToAdd, tb.maxTokens)
		tb.lastRefill = now
	}
	
	// Check if we have tokens available
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	
	return false
}

// GetTokens returns current token count
func (tb *APITokenBucket) GetTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	// Refill tokens first
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate
	
	if tokensToAdd > 0 {
		tb.tokens = min(tb.tokens+tokensToAdd, tb.maxTokens)
		tb.lastRefill = now
	}
	
	return tb.tokens
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}