package acp

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapError(t *testing.T) {
	t.Run("Wrap Standard Error", func(t *testing.T) {
		originalErr := errors.New("original error")
		wrapped := WrapError(originalErr, api.ErrorCodeInternalServerError, "operation failed")

		assert.Equal(t, api.ErrorCodeInternalServerError, wrapped.Code)
		assert.Contains(t, wrapped.Message, "operation failed")
		assert.Contains(t, wrapped.Message, "original error")

		// Check data contains original error
		data, ok := wrapped.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "original error", data["originalError"])
	})

	t.Run("Wrap Nil Error", func(t *testing.T) {
		wrapped := WrapError(nil, api.CodeInvalidParams, "validation failed")

		assert.Equal(t, api.CodeInvalidParams, wrapped.Code)
		assert.Equal(t, "validation failed", wrapped.Message)
		assert.Nil(t, wrapped.Data)
	})
}

func TestAsACPError(t *testing.T) {
	t.Run("Convert ACPError Pointer", func(t *testing.T) {
		original := &api.ACPError{
			Code:    api.ErrorCodeNotFound,
			Message: "not found",
		}

		converted, ok := AsACPError(original)
		assert.True(t, ok)
		assert.Equal(t, original, converted)
	})

	t.Run("Convert ACPError Value", func(t *testing.T) {
		original := api.ACPError{
			Code:    api.ErrorCodeForbidden,
			Message: "forbidden",
		}

		converted, ok := AsACPError(original)
		assert.True(t, ok)
		assert.Equal(t, original.Code, converted.Code)
		assert.Equal(t, original.Message, converted.Message)
	})

	t.Run("Convert Non-ACPError", func(t *testing.T) {
		err := errors.New("standard error")
		converted, ok := AsACPError(err)
		assert.False(t, ok)
		assert.Nil(t, converted)
	})

	t.Run("Convert Nil", func(t *testing.T) {
		converted, ok := AsACPError(nil)
		assert.False(t, ok)
		assert.Nil(t, converted)
	})
}

func TestContextError(t *testing.T) {
	t.Run("NewContextError", func(t *testing.T) {
		ctx := context.Background()
		err := NewContextError(ctx, api.ErrorCodeUnauthorized, "unauthorized access")

		assert.Equal(t, api.ErrorCodeUnauthorized, err.Code)
		assert.Equal(t, "unauthorized access", err.Message)
		assert.NotNil(t, err.Data)
	})

	t.Run("NewContextError with Nil Context", func(t *testing.T) {
		err := NewContextError(context.Background(), api.ErrorCodeInternalServerError, "internal error")

		assert.Equal(t, api.ErrorCodeInternalServerError, err.Code)
		assert.Equal(t, "internal error", err.Message)
	})

	t.Run("WithContext", func(t *testing.T) {
		original := &api.ACPError{
			Code:    api.ErrorCodeNotFound,
			Message: "not found",
		}

		// Add context
		withContext := WithContext(original, "resource", "user")
		withContext = WithContext(withContext, "id", "123")

		data, ok := withContext.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "user", data["resource"])
		assert.Equal(t, "123", data["id"])
	})

	t.Run("WithContext Nil Error", func(t *testing.T) {
		result := WithContext(nil, "key", "value")
		assert.Nil(t, result)
	})

	t.Run("WithContext Non-Map Data", func(t *testing.T) {
		original := &api.ACPError{
			Code:    api.ErrorCodeNotFound,
			Message: "not found",
			Data:    "string data",
		}

		withContext := WithContext(original, "key", "value")

		data, ok := withContext.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "string data", data["originalData"])
		assert.Equal(t, "value", data["key"])
	})
}

func TestFromStandardError(t *testing.T) {
	t.Run("Standard Error", func(t *testing.T) {
		err := errors.New("standard error")
		acpErr := FromStandardError(err)

		assert.Equal(t, api.ErrorCodeInternalServerError, acpErr.Code)
		assert.Equal(t, "standard error", acpErr.Message)
	})

	t.Run("ACPError", func(t *testing.T) {
		original := &api.ACPError{
			Code:    api.ErrorCodeNotFound,
			Message: "not found",
		}

		converted := FromStandardError(original)
		assert.Equal(t, original.Code, converted.Code)
		assert.Equal(t, original.Message, converted.Message)
	})

	t.Run("Nil Error", func(t *testing.T) {
		converted := FromStandardError(nil)
		assert.Nil(t, converted)
	})
}

func TestValidationHelpers(t *testing.T) {
	t.Run("NewValidationError", func(t *testing.T) {
		err := NewValidationError("email", "invalid format")

		assert.Equal(t, api.CodeInvalidParams, err.Code)
		assert.Contains(t, err.Message, "email")
		assert.Contains(t, err.Message, "invalid format")

		data, ok := err.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "email", data["field"])
		assert.Equal(t, "invalid format", data["reason"])
	})

	t.Run("NewNotFoundError", func(t *testing.T) {
		err := NewNotFoundError("user", "123")

		assert.Equal(t, api.ErrorCodeNotFound, err.Code)
		assert.Contains(t, err.Message, "user")
		assert.Contains(t, err.Message, "123")

		data, ok := err.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "user", data["resource"])
		assert.Equal(t, "123", data["id"])
	})

	t.Run("NewConflictError", func(t *testing.T) {
		err := NewConflictError("session", "already exists")

		assert.Equal(t, api.ErrorCodeConflict, err.Code)
		assert.Contains(t, err.Message, "session")
		assert.Contains(t, err.Message, "already exists")

		data, ok := err.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "session", data["resource"])
		assert.Equal(t, "already exists", data["reason"])
	})

	t.Run("NewRateLimitError", func(t *testing.T) {
		err := NewRateLimitError(100, "minute")

		assert.Equal(t, api.ErrorCodeTooManyRequests, err.Code)
		assert.Contains(t, err.Message, "100")
		assert.Contains(t, err.Message, "minute")

		data, ok := err.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 100, data["limit"])
		assert.Equal(t, "minute", data["window"])
	})
}

func TestGetDebugInfo(t *testing.T) {
	t.Run("Get Debug Info", func(t *testing.T) {
		err := &api.ACPError{
			Code:    api.ErrorCodeAuthRequired,
			Message: "auth required",
			Data:    map[string]string{"realm": "api"},
		}

		info := GetDebugInfo(err)
		assert.Equal(t, api.ErrorCodeAuthRequired, info["code"])
		assert.Equal(t, "auth required", info["message"])
		assert.Equal(t, err.Data, info["data"])
	})

	t.Run("Get Debug Info Nil", func(t *testing.T) {
		info := GetDebugInfo(nil)
		assert.Nil(t, info)
	})
}

func TestErrorChecks(t *testing.T) {
	t.Run("IsAuthRequired", func(t *testing.T) {
		authErr := &api.ACPError{Code: api.ErrorCodeAuthRequired, Message: "auth required"}
		otherErr := &api.ACPError{Code: api.ErrorCodeNotFound, Message: "not found"}
		standardErr := errors.New("standard error")

		assert.True(t, IsAuthRequired(authErr))
		assert.False(t, IsAuthRequired(otherErr))
		assert.False(t, IsAuthRequired(standardErr))
		assert.False(t, IsAuthRequired(nil))
	})

	t.Run("IsNotFound", func(t *testing.T) {
		notFoundErr := &api.ACPError{Code: api.ErrorCodeNotFound, Message: "not found"}
		otherErr := &api.ACPError{Code: api.ErrorCodeAuthRequired, Message: "auth required"}
		standardErr := errors.New("standard error")

		assert.True(t, IsNotFound(notFoundErr))
		assert.False(t, IsNotFound(otherErr))
		assert.False(t, IsNotFound(standardErr))
		assert.False(t, IsNotFound(nil))
	})

	t.Run("IsValidationError", func(t *testing.T) {
		validationErr := &api.ACPError{Code: api.CodeInvalidParams, Message: "invalid params"}
		otherErr := &api.ACPError{Code: api.ErrorCodeNotFound, Message: "not found"}
		standardErr := errors.New("standard error")

		assert.True(t, IsValidationError(validationErr))
		assert.False(t, IsValidationError(otherErr))
		assert.False(t, IsValidationError(standardErr))
		assert.False(t, IsValidationError(nil))
	})

	t.Run("IsInternalError", func(t *testing.T) {
		internalErr := &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "internal error"}
		otherErr := &api.ACPError{Code: api.ErrorCodeNotFound, Message: "not found"}
		standardErr := errors.New("standard error")

		assert.True(t, IsInternalError(internalErr))
		assert.False(t, IsInternalError(otherErr))
		assert.False(t, IsInternalError(standardErr))
		assert.False(t, IsInternalError(nil))
	})

	t.Run("IsRateLimitError", func(t *testing.T) {
		rateLimitErr := &api.ACPError{Code: api.ErrorCodeTooManyRequests, Message: "too many requests"}
		otherErr := &api.ACPError{Code: api.ErrorCodeNotFound, Message: "not found"}
		standardErr := errors.New("standard error")

		assert.True(t, IsRateLimitError(rateLimitErr))
		assert.False(t, IsRateLimitError(otherErr))
		assert.False(t, IsRateLimitError(standardErr))
		assert.False(t, IsRateLimitError(nil))
	})
}

func TestErrorFormatting(t *testing.T) {
	t.Run("ACPError Format", func(t *testing.T) {
		err := api.ACPError{
			Code:    api.ErrorCodeNotFound,
			Message: "resource not found",
		}

		errStr := err.Error()
		assert.Contains(t, errStr, strconv.Itoa(api.ErrorCodeNotFound))
		assert.Contains(t, errStr, "resource not found")
	})

	t.Run("ACPError Format with Data", func(t *testing.T) {
		err := api.ACPError{
			Code:    api.CodeInvalidParams,
			Message: "invalid parameters",
			Data:    map[string]string{"field": "email"},
		}

		errStr := err.Error()
		assert.Contains(t, errStr, strconv.Itoa(api.CodeInvalidParams))
		assert.Contains(t, errStr, "invalid parameters")
		assert.Contains(t, errStr, "data:")
	})
}
