package mcp

import "fmt"

// Error codes
const (
	ErrCodeUnauthorized    = -32001
	ErrCodeForbidden      = -32003
	ErrCodeNotFound       = -32004
	ErrCodeValidation     = -32005
	ErrCodeInternal       = -32006
	ErrCodeTimeout        = -32007
)

// Error codes as strings for error responses
const (
	ErrUnauthorized    = "UNAUTHORIZED"
	ErrForbidden       = "FORBIDDEN"
	ErrNotFound        = "NOT_FOUND"
	ErrValidation      = "VALIDATION_ERROR"
	ErrInternal        = "INTERNAL_ERROR"
	ErrTimeout         = "TIMEOUT"
)

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    string
	Message string
	Data    any
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *MCPError {
	return &MCPError{Code: ErrUnauthorized, Message: message}
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *MCPError {
	return &MCPError{Code: ErrForbidden, Message: message}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) *MCPError {
	return &MCPError{Code: ErrNotFound, Message: message}
}

// NewValidationError creates a validation error
func NewValidationError(message string) *MCPError {
	return &MCPError{Code: ErrValidation, Message: message}
}

// NewInternalError creates an internal error
func NewInternalError(message string) *MCPError {
	return &MCPError{Code: ErrInternal, Message: message}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string) *MCPError {
	return &MCPError{Code: ErrTimeout, Message: message}
}
