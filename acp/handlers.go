package acp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"golang.org/x/exp/jsonrpc2"
)

// Handler is the interface that wraps the Handle method.
// It's implemented by HandlerRegistry and used by AgentConnection to dispatch requests.
type Handler interface {
	Handle(ctx context.Context, conn *AgentConnection, req *jsonrpc2.Request) (interface{}, error)
}

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

// Handle dispatches incoming requests to the appropriate registered handler.
// It implements the Handler interface.
func (h *HandlerRegistry) Handle(ctx context.Context, _ *AgentConnection, req *jsonrpc2.Request) (any, error) {
	// Handle notifications.
	if !req.IsCall() {
		handler, exists := h.notifications[req.Method]
		if !exists {
			// No error response for unknown notifications
			return struct{}{}, nil
		}
		return nil, handler(ctx, req.Params)
	}

	// Handle method calls.
	handler, exists := h.methods[req.Method]
	if !exists {
		return nil, jsonrpc2.ErrMethodNotFound
	}

	return handler(ctx, req.Params)
}

// Typed handler registration helpers.

// RegisterInitializeHandler registers a typed handler for the initialize method.
func (h *HandlerRegistry) RegisterInitializeHandler(
	handler func(_ context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error),
) {
	h.RegisterMethod(api.MethodInitialize, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.InitializeRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		return handler(ctx, &params)
	})
}

// RegisterAuthenticateHandler registers a typed handler for the authenticate method.
func (h *HandlerRegistry) RegisterAuthenticateHandler(
	handler func(_ context.Context, params *api.AuthenticateRequest) error,
) {
	h.RegisterMethod(api.MethodAuthenticate, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.AuthenticateRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		err := handler(ctx, &params)
		if err != nil {
			return nil, err
		}
		return struct{}{}, nil
	})
}

// RegisterSessionNewHandler registers a typed handler for the session/new method.
func (h *HandlerRegistry) RegisterSessionNewHandler(
	handler func(_ context.Context, params *api.NewSessionRequest) (*api.NewSessionResponse, error),
) {
	h.RegisterMethod(api.MethodSessionNew, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.NewSessionRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		return handler(ctx, &params)
	})
}

// RegisterSessionLoadHandler registers a typed handler for the session/load method.
func (h *HandlerRegistry) RegisterSessionLoadHandler(
	handler func(_ context.Context, params *api.LoadSessionRequest) error,
) {
	h.RegisterMethod(api.MethodSessionLoad, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.LoadSessionRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		err := handler(ctx, &params)
		if err != nil {
			return nil, err
		}
		return struct{}{}, nil
	})
}

// RegisterSessionPromptHandler registers a typed handler for the session/prompt method.
func (h *HandlerRegistry) RegisterSessionPromptHandler(
	handler func(_ context.Context, params *api.PromptRequest) (*api.PromptResponse, error),
) {
	h.RegisterMethod(api.MethodSessionPrompt, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.PromptRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		return handler(ctx, &params)
	})
}

// Client-side method helpers.

// RegisterFsReadTextFileHandler registers a typed handler for the fs/read_text_file method.
func (h *HandlerRegistry) RegisterFsReadTextFileHandler(
	handler func(_ context.Context, params *api.ReadTextFileRequest) (*api.ReadTextFileResponse, error),
) {
	h.RegisterMethod(api.MethodFsReadTextFile, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.ReadTextFileRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		return handler(ctx, &params)
	})
}

// RegisterFsWriteTextFileHandler registers a typed handler for the fs/write_text_file method.
func (h *HandlerRegistry) RegisterFsWriteTextFileHandler(
	handler func(_ context.Context, params *api.WriteTextFileRequest) error,
) {
	h.RegisterMethod(api.MethodFsWriteTextFile, func(ctx context.Context, rawParams json.RawMessage) (any, error) {
		var params api.WriteTextFileRequest
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
		}
		err := handler(ctx, &params)
		if err != nil {
			return nil, err
		}
		return struct{}{}, nil
	})
}

// RegisterSessionRequestPermissionHandler registers a typed handler for the session/request_permission method.
func (h *HandlerRegistry) RegisterSessionRequestPermissionHandler(
	handler func(_ context.Context, params *api.RequestPermissionRequest) (*api.RequestPermissionResponse, error),
) {
	h.RegisterMethod(
		api.MethodSessionRequestPermission,
		func(ctx context.Context, rawParams json.RawMessage) (any, error) {
			var params api.RequestPermissionRequest
			if err := json.Unmarshal(rawParams, &params); err != nil {
				return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInvalidParams, err)
			}
			return handler(ctx, &params)
		},
	)
}

// Notification handlers.

// RegisterSessionUpdateHandler registers a typed handler for the session/update notification.
func (h *HandlerRegistry) RegisterSessionUpdateHandler(
	handler func(_ context.Context, params *api.SessionNotification) error,
) {
	h.RegisterNotification(api.MethodSessionUpdate, func(ctx context.Context, rawParams json.RawMessage) error {
		var params api.SessionNotification
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("invalid parameters: %w", err)
		}
		return handler(ctx, &params)
	})
}

// RegisterSessionCancelHandler registers a typed handler for the session/cancel notification.
func (h *HandlerRegistry) RegisterSessionCancelHandler(
	handler func(_ context.Context, params *api.CancelNotification) error,
) {
	h.RegisterNotification(api.MethodSessionCancel, func(ctx context.Context, rawParams json.RawMessage) error {
		var params api.CancelNotification
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("invalid parameters: %w", err)
		}
		return handler(ctx, &params)
	})
}
