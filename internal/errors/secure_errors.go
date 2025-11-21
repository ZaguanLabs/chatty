package errors

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// ErrorSecurityLevel defines the level of detail in error messages
type ErrorSecurityLevel int

const (
	// ErrorLevelDebug provides full error details for debugging
	ErrorLevelDebug ErrorSecurityLevel = iota
	// ErrorLevelInfo provides general error information
	ErrorLevelInfo
	// ErrorLevelProduction provides minimal, sanitized error messages
	ErrorLevelProduction
)

// Global error security level - should be set based on environment
var globalErrorSecurityLevel = ErrorLevelProduction

// SetErrorSecurityLevel sets the global error security level
func SetErrorSecurityLevel(level ErrorSecurityLevel) {
	globalErrorSecurityLevel = level
}

// GetErrorSecurityLevel returns the current error security level
func GetErrorSecurityLevel() ErrorSecurityLevel {
	return globalErrorSecurityLevel
}

// SecureError provides secure error handling with configurable detail levels
type SecureError struct {
	publicMessage  string
	detailMessage  string
	errorCode    string
	severity     string
	source       string
	cause        error
	stackTrace   []string
}

// NewSecureError creates a new secure error
func NewSecureError(publicMsg, detailMsg, errorCode, severity string, cause error) *SecureError {
	se := &SecureError{
		publicMessage: sanitizePublicMessage(publicMsg),
		detailMessage: detailMsg,
		errorCode:   errorCode,
		severity:    severity,
		cause:       cause,
	}
	
	// Capture stack trace for debugging
	if globalErrorSecurityLevel == ErrorLevelDebug {
		se.captureStackTrace()
	}
	
	return se
}

// Error returns the appropriate error message based on security level
func (se *SecureError) Error() string {
	switch globalErrorSecurityLevel {
	case ErrorLevelDebug:
		return se.getDebugMessage()
	case ErrorLevelInfo:
		return se.getInfoMessage()
	case ErrorLevelProduction:
		return se.getProductionMessage()
	default:
		return se.getProductionMessage()
	}
}

// getProductionMessage returns a sanitized message for production
func (se *SecureError) getProductionMessage() string {
	if se.publicMessage != "" {
		return se.publicMessage
	}
	return "An error occurred. Please try again or contact support if the issue persists."
}

// getInfoMessage returns a general information message
func (se *SecureError) getInfoMessage() string {
	if se.errorCode != "" {
		return fmt.Sprintf("%s (Code: %s)", se.publicMessage, se.errorCode)
	}
	return se.publicMessage
}

// getDebugMessage returns detailed information for debugging
func (se *SecureError) getDebugMessage() string {
	var parts []string
	
	if se.publicMessage != "" {
		parts = append(parts, fmt.Sprintf("Public: %s", se.publicMessage))
	}
	
	if se.detailMessage != "" {
		parts = append(parts, fmt.Sprintf("Detail: %s", se.detailMessage))
	}
	
	if se.errorCode != "" {
		parts = append(parts, fmt.Sprintf("Code: %s", se.errorCode))
	}
	
	if se.severity != "" {
		parts = append(parts, fmt.Sprintf("Severity: %s", se.severity))
	}
	
	if se.source != "" {
		parts = append(parts, fmt.Sprintf("Source: %s", se.source))
	}
	
	if se.cause != nil {
		parts = append(parts, fmt.Sprintf("Cause: %v", se.cause))
	}
	
	if len(se.stackTrace) > 0 {
		parts = append(parts, fmt.Sprintf("Stack: %s", strings.Join(se.stackTrace, " -> ")))
	}
	
	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}
	
	return "Unknown error"
}

// captureStackTrace captures the current stack trace
func (se *SecureError) captureStackTrace() {
	// Get the current stack trace (skip the first few frames)
	pc := make([]uintptr, 10)
	n := runtime.Callers(3, pc)
	
	se.stackTrace = make([]string, 0, n)
	for i := 0; i < n; i++ {
		fn := runtime.FuncForPC(pc[i])
		if fn != nil {
			file, line := fn.FileLine(pc[i])
			se.stackTrace = append(se.stackTrace, fmt.Sprintf("%s:%d", file, line))
		}
	}
}

// sanitizePublicMessage sanitizes error messages for public consumption
func sanitizePublicMessage(msg string) string {
	if msg == "" {
		return ""
	}
	
	// Remove potentially sensitive information
	sanitized := msg
	
	// Remove file paths
	sanitized = regexp.MustCompile(`[a-zA-Z]:\\[^\s]+|/[^\s]+`).ReplaceAllString(sanitized, "[PATH]")
	
	// Remove IP addresses
	sanitized = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`).ReplaceAllString(sanitized, "[IP]")
	
	// Remove email addresses
	sanitized = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`).ReplaceAllString(sanitized, "[EMAIL]")
	
	// Remove API keys and secrets
	sanitized = regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)["\s]*[:=]["\s]*[a-zA-Z0-9_-]+`).ReplaceAllString(sanitized, "[CREDENTIAL]")
	
	// Remove database connection strings
	sanitized = regexp.MustCompile(`(?i)(mongodb|mysql|postgres|redis)://[^\s]+`).ReplaceAllString(sanitized, "[DB_CONNECTION]")
	
	// Remove specific error details that might reveal system information
	sanitized = regexp.MustCompile(`(?i)(connection refused|permission denied|access denied|unauthorized|forbidden)`).ReplaceAllString(sanitized, "access issue")
	
	return sanitized
}

// Common secure error constructors

// NewSecureAPIError creates a secure API error
func NewSecureAPIError(publicMsg string, detailMsg string, code int, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		detailMsg,
		fmt.Sprintf("API_%d", code),
		"ERROR",
		cause,
	)
}

// NewSecureConfigError creates a secure configuration error
func NewSecureConfigError(publicMsg string, detailMsg string, field string, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		fmt.Sprintf("Config field %s: %s", field, detailMsg),
		"CONFIG_INVALID",
		"ERROR",
		cause,
	)
}

// NewSecureValidationError creates a secure validation error
func NewSecureValidationError(publicMsg string, detailMsg string, field string, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		fmt.Sprintf("Validation for %s: %s", field, detailMsg),
		"VALIDATION_FAILED",
		"WARNING",
		cause,
	)
}

// NewSecureNetworkError creates a secure network error
func NewSecureNetworkError(publicMsg string, detailMsg string, url string, status int, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		fmt.Sprintf("Network to %s (status %d): %s", url, status, detailMsg),
		"NETWORK_ERROR",
		"ERROR",
		cause,
	)
}

// NewSecureStorageError creates a secure storage error
func NewSecureStorageError(publicMsg string, detailMsg string, operation string, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		fmt.Sprintf("Storage during %s: %s", operation, detailMsg),
		fmt.Sprintf("STORAGE_%s", operation),
		"ERROR",
		cause,
	)
}

// NewSecureTimeoutError creates a secure timeout error
func NewSecureTimeoutError(publicMsg string, detailMsg string, operation string, duration string, cause error) *SecureError {
	return NewSecureError(
		sanitizePublicMessage(publicMsg),
		fmt.Sprintf("Timeout for %s (duration %s): %s", operation, duration, detailMsg),
		"TIMEOUT",
		"ERROR",
		cause,
	)
}