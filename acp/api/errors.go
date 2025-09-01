package api

import (
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// ACP-specific error codes (using the reserved range for application errors).
const (
	ErrorCodeAuthRequired        = -32000 // Authentication required (matches Rust/TypeScript)
	ErrorCodeInitializationError = -32001
	ErrorCodeUnauthorized        = -32002
	ErrorCodeForbidden           = -32003
	ErrorCodeNotFound            = -32004
	ErrorCodeConflict            = -32005
	ErrorCodeTooManyRequests     = -32006
	ErrorCodeInternalServerError = -32007
)

// ACPError represents an Agent Client Protocol error.
type ACPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewACPError creates a new ACP error with optional data.
func NewACPError(code int, message string, data any) *ACPError {
	return &ACPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (e ACPError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("ACP Error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("ACP Error %d: %s", e.Code, e.Message)
}

// Common ACP errors.
var (
	// Standard JSON-RPC 2.0 errors (using jsonrpc2 constants).
	ErrParseError     = ACPError{Code: int(jsonrpc2.CodeParseError), Message: "Parse error"}
	ErrInvalidRequest = ACPError{Code: int(jsonrpc2.CodeInvalidRequest), Message: "Invalid request"}
	ErrMethodNotFound = ACPError{Code: int(jsonrpc2.CodeMethodNotFound), Message: "Method not found"}
	ErrInvalidParams  = ACPError{Code: int(jsonrpc2.CodeInvalidParams), Message: "Invalid params"}
	ErrInternalError  = ACPError{Code: int(jsonrpc2.CodeInternalError), Message: "Internal error"}

	// ACP-specific errors.
	ErrAuthRequired        = ACPError{Code: ErrorCodeAuthRequired, Message: "Authentication required"}
	ErrInitializationError = ACPError{Code: ErrorCodeInitializationError, Message: "Initialization error"}
	ErrUnauthorized        = ACPError{Code: ErrorCodeUnauthorized, Message: "Unauthorized"}
	ErrForbidden           = ACPError{Code: ErrorCodeForbidden, Message: "Forbidden"}
	ErrNotFound            = ACPError{Code: ErrorCodeNotFound, Message: "Not found"}
	ErrConflict            = ACPError{Code: ErrorCodeConflict, Message: "Conflict"}
	ErrTooManyRequests     = ACPError{Code: ErrorCodeTooManyRequests, Message: "Too many requests"}
	ErrInternalServerError = ACPError{Code: ErrorCodeInternalServerError, Message: "Internal server error"}
)
