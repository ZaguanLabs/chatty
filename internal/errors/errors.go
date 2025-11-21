package errors

import "fmt"

// Base error interface that all custom errors implement
type ChattyError interface {
	error
	Type() string
	Code() string
	Cause() error
}

// APIError represents errors from the OpenAI-compatible API
type APIError struct {
	code    int    `json:"code"`
	message string `json:"message"`
	errType string `json:"type"`
	cause   error  `json:"-"`
}

func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("API error (code %d, type %s): %s (caused by: %v)", e.code, e.errType, e.message, e.cause)
	}
	return fmt.Sprintf("API error (code %d, type %s): %s", e.code, e.errType, e.message)
}

func (e *APIError) Type() string { return "API" }
func (e *APIError) Code() string { return fmt.Sprintf("API_%d", e.code) }
func (e *APIError) Cause() error { return e.cause }

// ConfigError represents configuration-related errors
type ConfigError struct {
	field   string
	message string
	cause   error
}

func (e *ConfigError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("Config error in field %q: %s (caused by: %v)", e.field, e.message, e.cause)
	}
	return fmt.Sprintf("Config error in field %q: %s", e.field, e.message)
}

func (e *ConfigError) Type() string { return "Config" }
func (e *ConfigError) Code() string { return "CONFIG_INVALID" }
func (e *ConfigError) Cause() error { return e.cause }

// ValidationError represents input validation errors
type ValidationError struct {
	field   string
	message string
	value   interface{}
	cause   error
}

func (e *ValidationError) Error() string {
	if e.value != nil {
		if e.cause != nil {
			return fmt.Sprintf("Validation error for field %q with value %v: %s (caused by: %v)", e.field, e.value, e.message, e.cause)
		}
		return fmt.Sprintf("Validation error for field %q with value %v: %s", e.field, e.value, e.message)
	}
	if e.cause != nil {
		return fmt.Sprintf("Validation error for field %q: %s (caused by: %v)", e.field, e.message, e.cause)
	}
	return fmt.Sprintf("Validation error for field %q: %s", e.field, e.message)
}

func (e *ValidationError) Type() string { return "Validation" }
func (e *ValidationError) Code() string { return "VALIDATION_FAILED" }
func (e *ValidationError) Cause() error { return e.cause }

// StorageError represents database/storage-related errors
type StorageError struct {
	operation string
	message   string
	cause     error
}

func (e *StorageError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("Storage error during %s operation: %s (caused by: %v)", e.operation, e.message, e.cause)
	}
	return fmt.Sprintf("Storage error during %s operation: %s", e.operation, e.message)
}

func (e *StorageError) Type() string { return "Storage" }
func (e *StorageError) Code() string { return fmt.Sprintf("STORAGE_%s", e.operation) }
func (e *StorageError) Cause() error { return e.cause }

// NetworkError represents network connectivity errors
type NetworkError struct {
	url     string
	message string
	status  int
	cause   error
}

func (e *NetworkError) Error() string {
	if e.status > 0 {
		if e.cause != nil {
			return fmt.Sprintf("Network error to %s (status %d): %s (caused by: %v)", e.url, e.status, e.message, e.cause)
		}
		return fmt.Sprintf("Network error to %s (status %d): %s", e.url, e.status, e.message)
	}
	if e.cause != nil {
		return fmt.Sprintf("Network error to %s: %s (caused by: %v)", e.url, e.message, e.cause)
	}
	return fmt.Sprintf("Network error to %s: %s", e.url, e.message)
}

func (e *NetworkError) Type() string { return "Network" }
func (e *NetworkError) Code() string { return "NETWORK_ERROR" }
func (e *NetworkError) Cause() error { return e.cause }

// TimeoutError represents timeout-related errors
type TimeoutError struct {
	operation string
	duration  string
	cause     error
}

func (e *TimeoutError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("Timeout error for %s operation (duration %s): %v", e.operation, e.duration, e.cause)
	}
	return fmt.Sprintf("Timeout error for %s operation (duration %s)", e.operation, e.duration)
}

func (e *TimeoutError) Type() string { return "Timeout" }
func (e *TimeoutError) Code() string { return "TIMEOUT" }
func (e *TimeoutError) Cause() error { return e.cause }

// CommandError represents command processing errors
type CommandError struct {
	command string
	message string
	cause   error
}

func (e *CommandError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("Command error for %q: %s (caused by: %v)", e.command, e.message, e.cause)
	}
	return fmt.Sprintf("Command error for %q: %s", e.command, e.message)
}

func (e *CommandError) Type() string { return "Command" }
func (e *CommandError) Code() string { return fmt.Sprintf("CMD_%s", e.command) }
func (e *CommandError) Cause() error { return e.cause }

// SessionError represents session management errors
type SessionError struct {
	sessionID int64
	message   string
	cause     error
}

func (e *SessionError) Error() string {
	if e.sessionID > 0 {
		if e.cause != nil {
			return fmt.Sprintf("Session error (ID %d): %s (caused by: %v)", e.sessionID, e.message, e.cause)
		}
		return fmt.Sprintf("Session error (ID %d): %s", e.sessionID, e.message)
	}
	if e.cause != nil {
		return fmt.Sprintf("Session error: %s (caused by: %v)", e.message, e.cause)
	}
	return fmt.Sprintf("Session error: %s", e.message)
}

func (e *SessionError) Type() string { return "Session" }
func (e *SessionError) Code() string { return "SESSION_ERROR" }
func (e *SessionError) Cause() error { return e.cause }

// Convenience constructors

// NewAPIError creates a new API error
func NewAPIError(code int, msg, errType string, cause error) *APIError {
	return &APIError{
		code:    code,
		message: msg,
		errType: errType,
		cause:   cause,
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(field, msg string, cause error) *ConfigError {
	return &ConfigError{
		field:   field,
		message: msg,
		cause:   cause,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(field, msg string, value interface{}, cause error) *ValidationError {
	return &ValidationError{
		field:   field,
		message: msg,
		value:   value,
		cause:   cause,
	}
}

// NewStorageError creates a new storage error
func NewStorageError(operation, msg string, cause error) *StorageError {
	return &StorageError{
		operation: operation,
		message:   msg,
		cause:     cause,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(url, msg string, status int, cause error) *NetworkError {
	return &NetworkError{
		url:     url,
		message: msg,
		status:  status,
		cause:   cause,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation, duration string, cause error) *TimeoutError {
	return &TimeoutError{
		operation: operation,
		duration:  duration,
		cause:     cause,
	}
}

// NewCommandError creates a new command error
func NewCommandError(command, msg string, cause error) *CommandError {
	return &CommandError{
		command: command,
		message: msg,
		cause:   cause,
	}
}

// NewSessionError creates a new session error
func NewSessionError(sessionID int64, msg string, cause error) *SessionError {
	return &SessionError{
		sessionID: sessionID,
		message:   msg,
		cause:     cause,
	}
}

// Error unwrapping helper - extracts the root cause
func Unwrap(err error) error {
	for {
		unwrapped, ok := err.(interface{ Cause() error })
		if !ok {
			break
		}
		cause := unwrapped.Cause()
		if cause == nil {
			break
		}
		err = cause
	}
	return err
}