package acp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// HandlerFunc represents a handler function for ACP methods.
type HandlerFunc func(_ context.Context, params json.RawMessage) (any, error)

// NotificationHandlerFunc represents a handler function for ACP notifications.
type NotificationHandlerFunc func(_ context.Context, params json.RawMessage) error

// HandlerRegistry manages method and notification handlers.
type HandlerRegistry struct {
	methods       map[string]HandlerFunc
	notifications map[string]NotificationHandlerFunc
}

// NewHandlerRegistry creates a new handler registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		methods:       make(map[string]HandlerFunc),
		notifications: make(map[string]NotificationHandlerFunc),
	}
}

// RegisterMethod registers a handler for a method (request/response).
func (h *HandlerRegistry) RegisterMethod(method string, handler HandlerFunc) {
	h.methods[method] = handler
}

// RegisterNotification registers a handler for a notification.
func (h *HandlerRegistry) RegisterNotification(method string, handler NotificationHandlerFunc) {
	h.notifications[method] = handler
}

// Handler returns a jsonrpc2.Handler that routes requests to registered handlers.
func (h *HandlerRegistry) Handler() jsonrpc2.Handler {
	return jsonrpc2.HandlerWithError(
		func(ctx context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
			// Handle notifications.
			if req.Notif {
				handler, exists := h.notifications[req.Method]
				if !exists {
					// Notifications don't return errors to the client,
					// just log internally.
					return nil, nil
				}

				params := json.RawMessage{}
				if req.Params != nil {
					params = *req.Params
				}
				return nil, handler(ctx, params)
			}

			// Handle method calls.
			handler, exists := h.methods[req.Method]
			if !exists {
				return nil, &jsonrpc2.Error{
					Code:    jsonrpc2.CodeMethodNotFound,
					Message: fmt.Sprintf("Method not found: %s", req.Method),
				}
			}

			params := json.RawMessage{}
			if req.Params != nil {
				params = *req.Params
			}
			return handler(ctx, params)
		},
	)
}

// Typed handler registration helpers.

// RegisterInitializeHandler registers a typed handler for the initialize method.
func (h *HandlerRegistry) RegisterInitializeHandler(
	handler func(_ context.Context, params *InitializeRequest) (*InitializeResponse, error),
) {
	h.RegisterMethod(MethodInitialize, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params InitializeRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return handler(ctx, &params)
	})
}

// RegisterAuthenticateHandler registers a typed handler for the authenticate method.
func (h *HandlerRegistry) RegisterAuthenticateHandler(
	handler func(_ context.Context, params *AuthenticateRequest) error,
) {
	h.RegisterMethod(MethodAuthenticate, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params AuthenticateRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return nil, handler(ctx, &params)
	})
}

// RegisterSessionNewHandler registers a typed handler for the session/new method.
func (h *HandlerRegistry) RegisterSessionNewHandler(
	handler func(_ context.Context, params *NewSessionRequest) (*NewSessionResponse, error),
) {
	h.RegisterMethod(MethodSessionNew, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params NewSessionRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return handler(ctx, &params)
	})
}

// RegisterSessionLoadHandler registers a typed handler for the session/load method.
func (h *HandlerRegistry) RegisterSessionLoadHandler(
	handler func(_ context.Context, params *LoadSessionRequest) error,
) {
	h.RegisterMethod(MethodSessionLoad, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params LoadSessionRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return nil, handler(ctx, &params)
	})
}

// RegisterSessionPromptHandler registers a typed handler for the session/prompt method.
func (h *HandlerRegistry) RegisterSessionPromptHandler(
	handler func(_ context.Context, params *PromptRequest) (*PromptResponse, error),
) {
	h.RegisterMethod(MethodSessionPrompt, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params PromptRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return handler(ctx, &params)
	})
}

// Client-side method helpers.

// RegisterFsReadTextFileHandler registers a typed handler for the fs/read_text_file method.
func (h *HandlerRegistry) RegisterFsReadTextFileHandler(
	handler func(_ context.Context, params *ReadTextFileRequest) (*ReadTextFileResponse, error),
) {
	h.RegisterMethod(MethodFsReadTextFile, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params ReadTextFileRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return handler(ctx, &params)
	})
}

// RegisterFsWriteTextFileHandler registers a typed handler for the fs/write_text_file method.
func (h *HandlerRegistry) RegisterFsWriteTextFileHandler(
	handler func(_ context.Context, params *WriteTextFileRequest) error,
) {
	h.RegisterMethod(MethodFsWriteTextFile, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params WriteTextFileRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Invalid parameters"}
		}
		return nil, handler(ctx, &params)
	})
}
