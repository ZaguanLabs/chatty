package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

// RandomGenerator provides cryptographically secure random number generation
type RandomGenerator struct{}

// NewRandomGenerator creates a new secure random generator
func NewRandomGenerator() *RandomGenerator {
	return &RandomGenerator{}
}

// GenerateSecureInt64 generates a cryptographically secure random int64
func (rg *RandomGenerator) GenerateSecureInt64(min, max int64) (int64, error) {
	if min >= max {
		return 0, fmt.Errorf("invalid range: min must be less than max")
	}
	
	// Calculate the range
	rangeSize := new(big.Int).SetInt64(max - min)
	
	// Generate a random number within the range
	randomNum, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return 0, fmt.Errorf("failed to generate secure random number: %w", err)
	}
	
	// Add the minimum value to shift the range
	return randomNum.Int64() + min, nil
}

// GenerateSecureBytes generates cryptographically secure random bytes
func (rg *RandomGenerator) GenerateSecureBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("invalid length: must be positive")
	}
	
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	
	return bytes, nil
}

// GenerateSecureHex generates a cryptographically secure random hex string
func (rg *RandomGenerator) GenerateSecureHex(length int) (string, error) {
	bytes, err := rg.GenerateSecureBytes(length)
	if err != nil {
		return "", err
	}
	
	return hex.EncodeToString(bytes), nil
}

// GenerateSecureSessionID generates a cryptographically secure session ID
func (rg *RandomGenerator) GenerateSecureSessionID() (int64, error) {
	// Generate a positive int64 for session IDs
	// Use a reasonable range to avoid database issues
	const (
		minSessionID = 1000000  // Minimum session ID
		maxSessionID = 9000000  // Maximum session ID
	)
	
	return rg.GenerateSecureInt64(minSessionID, maxSessionID)
}

// GenerateSecureToken generates a cryptographically secure token
func (rg *RandomGenerator) GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid token length: must be positive")
	}
	
	// Generate twice the length since hex encoding doubles the size
	bytes, err := rg.GenerateSecureBytes(length / 2)
	if err != nil {
		return "", err
	}
	
	return hex.EncodeToString(bytes), nil
}

// GenerateSecurePassword generates a cryptographically secure password
func (rg *RandomGenerator) GenerateSecurePassword(length int) (string, error) {
	if length < 8 {
		return "", fmt.Errorf("password length must be at least 8 characters")
	}
	
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"
	
	bytes, err := rg.GenerateSecureBytes(length)
	if err != nil {
		return "", err
	}
	
	password := make([]byte, length)
	for i := 0; i < length; i++ {
		// Use modulo to map random byte to charset
		password[i] = charset[bytes[i]%byte(len(charset))]
	}
	
	return string(password), nil
}

// GenerateSecureNonce generates a cryptographically secure nonce
func (rg *RandomGenerator) GenerateSecureNonce() (string, error) {
	// Generate a 16-byte nonce (128 bits)
	return rg.GenerateSecureHex(16)
}

// GenerateSecureCorrelationID generates a secure correlation ID for request tracking
func (rg *RandomGenerator) GenerateSecureCorrelationID() (string, error) {
	// Generate a 8-byte correlation ID (64 bits)
	return rg.GenerateSecureHex(8)
}