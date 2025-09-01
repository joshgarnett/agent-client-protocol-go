package acp

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// Error wrapping utilities

// WrapError wraps an error with an ACP error code and message.
func WrapError(err error, code int, message string) *api.ACPError {
	if err == nil {
		return api.NewACPError(code, message, nil)
	}

	data := map[string]interface{}{
		"originalError": err.Error(),
	}

	return api.NewACPError(code, fmt.Sprintf("%s: %v", message, err), data)
}

// AsACPError attempts to convert an error to an ACP error.
func AsACPError(err error) (*api.ACPError, bool) {
	if err == nil {
		return nil, false
	}

	// Check if it's already an ACP error pointer
	var acpErrPtr *api.ACPError
	if errors.As(err, &acpErrPtr) {
		return acpErrPtr, true
	}

	// Check if it's an ACP error value
	var acpErrVal api.ACPError
	if errors.As(err, &acpErrVal) {
		return &acpErrVal, true
	}

	return nil, false
}

// Context-aware errors

// NewContextError creates an error with context information.
func NewContextError(ctx context.Context, code int, message string) *api.ACPError {
	data := make(map[string]interface{})

	// Add context values if available
	// This is a placeholder for future context value extraction
	_ = ctx

	return api.NewACPError(code, message, data)
}

// WithContext adds context information to an existing error.
func WithContext(err *api.ACPError, key string, value interface{}) *api.ACPError {
	if err == nil {
		return nil
	}

	// Ensure data is a map
	var data map[string]interface{}
	if err.Data != nil {
		if m, ok := err.Data.(map[string]interface{}); ok {
			data = m
		} else {
			data = map[string]interface{}{
				"originalData": err.Data,
			}
		}
	} else {
		data = make(map[string]interface{})
	}

	data[key] = value

	return api.NewACPError(err.Code, err.Message, data)
}

// Standard error conversions

// FromStandardError converts a standard Go error to an ACP error.
func FromStandardError(err error) *api.ACPError {
	if err == nil {
		return nil
	}

	// Check if it's already an ACP error
	if acpErr, ok := AsACPError(err); ok {
		return acpErr
	}

	// Convert to internal error
	return api.NewACPError(api.ErrorCodeInternalServerError, err.Error(), nil)
}

// Validation helpers

// NewValidationError creates a validation error for a specific field.
func NewValidationError(field string, reason string) *api.ACPError {
	data := map[string]interface{}{
		"field":  field,
		"reason": reason,
	}

	message := fmt.Sprintf("Validation failed for field '%s': %s", field, reason)
	return api.NewACPError(api.CodeInvalidParams, message, data)
}

// NewNotFoundError creates a not found error for a resource.
func NewNotFoundError(resource string, id string) *api.ACPError {
	data := map[string]interface{}{
		"resource": resource,
		"id":       id,
	}

	message := fmt.Sprintf("%s with ID '%s' not found", resource, id)
	return api.NewACPError(api.ErrorCodeNotFound, message, data)
}

// NewConflictError creates a conflict error.
func NewConflictError(resource string, reason string) *api.ACPError {
	data := map[string]interface{}{
		"resource": resource,
		"reason":   reason,
	}

	message := fmt.Sprintf("Conflict in %s: %s", resource, reason)
	return api.NewACPError(api.ErrorCodeConflict, message, data)
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(limit int, window string) *api.ACPError {
	data := map[string]interface{}{
		"limit":  limit,
		"window": window,
	}

	message := fmt.Sprintf("Rate limit exceeded: %d requests per %s", limit, window)
	return api.NewACPError(api.ErrorCodeTooManyRequests, message, data)
}

// Error chain support

// ChainError represents a chain of errors.
type ChainError struct {
	errors []error
}

// NewErrorChain creates a new error chain.
func NewErrorChain() *ChainError {
	return &ChainError{
		errors: make([]error, 0),
	}
}

// Add adds an error to the chain.
func (ec *ChainError) Add(err error) *ChainError {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
	return ec
}

// AddAll adds multiple errors to the chain.
func (ec *ChainError) AddAll(errors []error) *ChainError {
	for _, err := range errors {
		_ = ec.Add(err)
	}
	return ec
}

// HasErrors returns true if the chain contains any errors.
func (ec *ChainError) HasErrors() bool {
	return len(ec.errors) > 0
}

// Count returns the number of errors in the chain.
func (ec *ChainError) Count() int {
	return len(ec.errors)
}

// ToACPError converts the error chain to an ACP error.
func (ec *ChainError) ToACPError() *api.ACPError {
	if !ec.HasErrors() {
		return nil
	}

	if len(ec.errors) == 1 {
		return FromStandardError(ec.errors[0])
	}

	// Build error messages
	var messages []string
	var details []map[string]interface{}

	for i, err := range ec.errors {
		messages = append(messages, fmt.Sprintf("%d. %v", i+1, err))

		detail := map[string]interface{}{
			"index":   i,
			"message": err.Error(),
		}

		// Add ACP error details if available
		if acpErr, ok := AsACPError(err); ok {
			detail["code"] = acpErr.Code
			if acpErr.Data != nil {
				detail["data"] = acpErr.Data
			}
		}

		details = append(details, detail)
	}

	data := map[string]interface{}{
		"count":  len(ec.errors),
		"errors": details,
	}

	message := fmt.Sprintf("Multiple errors occurred: %s", strings.Join(messages, "; "))
	return api.NewACPError(api.ErrorCodeInternalServerError, message, data)
}

// Error returns the error string representation.
func (ec *ChainError) Error() string {
	if !ec.HasErrors() {
		return ""
	}

	var messages []string
	for _, err := range ec.errors {
		messages = append(messages, err.Error())
	}

	return strings.Join(messages, "; ")
}

// Debugging support

const (
	stackTraceDepth   = 32
	stackCallerOffset = 2
)

// WithStackTrace adds a stack trace to an ACP error.
func WithStackTrace(err *api.ACPError) *api.ACPError {
	if err == nil {
		return nil
	}

	// Capture stack trace
	var pcs [stackTraceDepth]uintptr
	n := runtime.Callers(stackCallerOffset, pcs[:])

	var stack []string
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}

	return WithContext(err, "stackTrace", stack)
}

// GetDebugInfo extracts debug information from an ACP error.
func GetDebugInfo(err *api.ACPError) map[string]interface{} {
	if err == nil {
		return nil
	}

	info := map[string]interface{}{
		"code":    err.Code,
		"message": err.Message,
	}

	if err.Data != nil {
		info["data"] = err.Data
	}

	return info
}

// Common error checks

// IsAuthRequired checks if an error is an authentication required error.
func IsAuthRequired(err error) bool {
	acpErr, ok := AsACPError(err)
	return ok && acpErr.Code == api.ErrorCodeAuthRequired
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	acpErr, ok := AsACPError(err)
	return ok && acpErr.Code == api.ErrorCodeNotFound
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	acpErr, ok := AsACPError(err)
	return ok && acpErr.Code == api.CodeInvalidParams
}

// IsInternalError checks if an error is an internal server error.
func IsInternalError(err error) bool {
	acpErr, ok := AsACPError(err)
	return ok && acpErr.Code == api.ErrorCodeInternalServerError
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	acpErr, ok := AsACPError(err)
	return ok && acpErr.Code == api.ErrorCodeTooManyRequests
}
