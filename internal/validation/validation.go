package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Validation constants
const (
	MaxInputLength        = 100000  // 100KB max input
	MaxCommandLength      = 1000    // 1KB max command
	MaxUserMessageLength  = 50000   // 50KB max user message
	MinInputLength        = 1
	MaxIdentifierLength   = 200
	MaxPathLength          = 500
)

// Validation patterns
var (
	// Command validation - only allow specific characters
	CommandPattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_./:@#]+$`)
	
	// Safe identifier pattern (alphanumeric, underscore, hyphen)
	IdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	
	// Path pattern - safe file/directory paths
	PathPattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_./\\:@#]+$`)
	
	// URL pattern - basic URL validation
	URLPattern = regexp.MustCompile(`^https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=]+$`)
	
	// Model name pattern
	ModelNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-._/]+$`)
	
	// Temperature pattern - valid temperature values
	TemperaturePattern = regexp.MustCompile(`^(0(\.\d{1,2})?|1(\.0{1,2})?|2(\.0{0,2})?)$`)
	
	// Dangerous pattern detection
	SQLInjectionPattern = regexp.MustCompile(`(?i)(union\s+select|select\s+\*|insert\s+into|update\s+set|delete\s+from|drop\s+table|create\s+table|alter\s+table|exec\s*\(|script\s*\(|javascript:|vbscript:)`)
	
	XSSPattern = regexp.MustCompile(`(?i)(<script|<iframe|<object|<embed|<form|javascript:|onerror=|onload=|onclick=|onmouseover=|onfocus=|onblur=)`)
	
	CommandInjectionPattern = regexp.MustCompile(`(?i)(;|\|\||&&|\$\(|\$\{|<\(|>\(|\n|\r)`)
	
	PathTraversalPattern = regexp.MustCompile(`(\.\./|\.\.\\\\)`)
	
	// Common dangerous file extensions
	DangerousExtensionPattern = regexp.MustCompile(`\.(exe|scr|vbs|bat|cmd|com|pif|jar|apk|deb|rpm|msi|dmg|pkg|sh|bash|zsh|fish|ps1|psm1|dll|so|dylib)$`)
)

// Validation functions

// ValidateCommand validates command input
func ValidateCommand(input string) error {
	if input == "" {
		return errors.New("command cannot be empty")
	}
	
	if len(input) > MaxCommandLength {
		return fmt.Errorf("command too long (max %d characters)", MaxCommandLength)
	}
	
	// Check against command pattern
	if !CommandPattern.MatchString(input) {
		return errors.New("command contains invalid characters")
	}
	
	// Check for command injection
	if CommandInjectionPattern.MatchString(input) {
		return errors.New("command appears to contain injection attempt")
	}
	
	return nil
}

// ValidateUserInput validates general user input
func ValidateUserInput(input string, maxLength int) error {
	if input == "" {
		return errors.New("input cannot be empty")
	}
	
	if len(input) > maxLength {
		return fmt.Errorf("input too long (max %d characters)", maxLength)
	}
	
	if len(input) < MinInputLength {
		return fmt.Errorf("input too short (min %d characters)", MinInputLength)
	}
	
	// Check for SQL injection
	if SQLInjectionPattern.MatchString(input) {
		return errors.New("input appears to contain SQL injection attempt")
	}
	
	// Check for XSS
	if XSSPattern.MatchString(input) {
		return errors.New("input appears to contain XSS attempt")
	}
	
	// Check for path traversal
	if PathTraversalPattern.MatchString(input) {
		return errors.New("input appears to contain path traversal attempt")
	}
	
	// Remove null bytes
	if strings.Contains(input, "\x00") {
		return errors.New("input contains null bytes")
	}
	
	return nil
}

// ValidateMessage validates chat messages
func ValidateMessage(message string) error {
	if err := ValidateUserInput(message, MaxUserMessageLength); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}
	
	// Additional message-specific validation
	if strings.Count(message, "\n") > 1000 {
		return errors.New("message contains too many newlines")
	}
	
	// Check for excessive repetition (potential DoS)
	if hasExcessiveRepetition(message) {
		return errors.New("message contains excessive repetition")
	}
	
	return nil
}

// ValidateIdentifier validates identifiers (usernames, IDs, etc.)
func ValidateIdentifier(identifier string) error {
	if identifier == "" {
		return errors.New("identifier cannot be empty")
	}
	
	if len(identifier) > MaxIdentifierLength {
		return fmt.Errorf("identifier too long (max %d characters)", MaxIdentifierLength)
	}
	
	if !IdentifierPattern.MatchString(identifier) {
		return errors.New("identifier contains invalid characters (only alphanumeric, -, _ allowed)")
	}
	
	return nil
}

// ValidatePath validates file/directory paths
func ValidatePath(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	
	if len(path) > MaxPathLength {
		return fmt.Errorf("path too long (max %d characters)", MaxPathLength)
	}
	
	if !PathPattern.MatchString(path) {
		return errors.New("path contains invalid characters")
	}
	
	if PathTraversalPattern.MatchString(path) {
		return errors.New("path contains directory traversal attempt")
	}
	
	if DangerousExtensionPattern.MatchString(path) {
		return errors.New("path contains potentially dangerous file extension")
	}
	
	return nil
}

// ValidateURL validates URLs
func ValidateURL(url string) error {
	if url == "" {
		return errors.New("URL cannot be empty")
	}
	
	if len(url) > 1000 { // Reasonable max URL length
		return errors.New("URL too long")
	}
	
	if !URLPattern.MatchString(url) {
		return errors.New("invalid URL format")
	}
	
	return nil
}

// ValidateModelName validates AI model names
func ValidateModelName(model string) error {
	if model == "" {
		return errors.New("model name cannot be empty")
	}
	
	if len(model) > 200 {
		return errors.New("model name too long (max 200 characters)")
	}
	
	if !ModelNamePattern.MatchString(model) {
		return errors.New("model name contains invalid characters")
	}
	
	return nil
}

// ValidateTemperature validates temperature parameter
func ValidateTemperature(temp float64) error {
	if temp < 0.0 || temp > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got %.2f", temp)
	}
	
	return nil
}

// SanitizeInput performs basic input sanitization
func SanitizeInput(input string, maxLength int) string {
	if input == "" {
		return ""
	}
	
	// Trim whitespace
	trimmed := strings.TrimSpace(input)
	
	// Limit length
	if len(trimmed) > maxLength {
		trimmed = trimmed[:maxLength]
	}
	
	// Remove null bytes
	trimmed = strings.ReplaceAll(trimmed, "\x00", "")
	
	// Replace multiple spaces with single space
	trimmed = regexp.MustCompile(`\s+`).ReplaceAllString(trimmed, " ")
	
	return trimmed
}

// hasExcessiveRepetition checks if input contains excessive repetition
func hasExcessiveRepetition(input string) bool {
	if len(input) < 10 {
		return false
	}
	
	// Check for repeated characters
	repeatedCharThreshold := len(input) / 3
	for _, char := range input {
		if charCount := strings.Count(input, string(char)); charCount > repeatedCharThreshold {
			return true
		}
	}
	
	// Check for repeated substrings (simple check for 3+ character repeats)
	for i := 0; i < len(input)-3; i++ {
		substr := input[i : i+3]
		if strings.Count(input, substr) > len(input)/5 {
			return true
		}
	}
	
	return false
}

// IsPrintable checks if string contains only printable characters
func IsPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

// ValidatePrintable validates that input contains only printable characters
func ValidatePrintable(input string) error {
	if !IsPrintable(input) {
		return errors.New("input contains non-printable characters")
	}
	return nil
}