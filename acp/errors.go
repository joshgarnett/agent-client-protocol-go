package acp

import (
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// ACP-specific error codes (using the reserved range for application errors).
const (
	ErrorCodeInitializationError = -32000
	ErrorCodeUnauthorized        = -32001
	ErrorCodeForbidden           = -32002
	ErrorCodeNotFound            = -32003
	ErrorCodeConflict            = -32004
	ErrorCodeTooManyRequests     = -32005
	ErrorCodeInternalServerError = -32006
)

// ACPError represents an Agent Client Protocol error.
//
//nolint:revive // ACPError is more descriptive than Error for this specific protocol
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
	ErrInitializationError = ACPError{Code: ErrorCodeInitializationError, Message: "Initialization error"}
	ErrUnauthorized        = ACPError{Code: ErrorCodeUnauthorized, Message: "Unauthorized"}
	ErrForbidden           = ACPError{Code: ErrorCodeForbidden, Message: "Forbidden"}
	ErrNotFound            = ACPError{Code: ErrorCodeNotFound, Message: "Not found"}
	ErrConflict            = ACPError{Code: ErrorCodeConflict, Message: "Conflict"}
	ErrTooManyRequests     = ACPError{Code: ErrorCodeTooManyRequests, Message: "Too many requests"}
	ErrInternalServerError = ACPError{Code: ErrorCodeInternalServerError, Message: "Internal server error"}
)
