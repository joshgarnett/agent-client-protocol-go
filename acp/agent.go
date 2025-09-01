package acp

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// Errors for connection and notification handling.
var (
	ErrConnectionClosed = errors.New("connection is closed")
)

// AgentConnection represents a connection from an agent to a client.
// It abstracts away the underlying jsonrpc2 library and provides a safe way
// to make re-entrant calls from handlers.
type AgentConnection struct {
	core *ConnectionCore
}

// NewAgentConnectionStdio creates a new agent connection using stdio transport.
func NewAgentConnectionStdio(
	ctx context.Context,
	rwc io.ReadWriteCloser,
	handler Handler,
	timeout time.Duration,
) (*AgentConnection, error) {
	core, err := NewConnectionCore(ctx, rwc, handler, timeout)
	if err != nil {
		return nil, err
	}

	return &AgentConnection{core: core}, nil
}

// Close closes the connection.
func (a *AgentConnection) Close() error {
	return a.core.Close()
}

// Wait waits for the connection to close.
func (a *AgentConnection) Wait() error {
	return a.core.Wait()
}

// Agent method helpers.

// Initialize sends an initialize request to the client.
func (a *AgentConnection) Initialize(
	ctx context.Context,
	params *api.InitializeRequest,
) (*api.InitializeResponse, error) {
	var result api.InitializeResponse
	err := a.core.Call(ctx, api.MethodInitialize, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Authenticate sends an authenticate request to the client.
func (a *AgentConnection) Authenticate(ctx context.Context, params *api.AuthenticateRequest) error {
	return a.core.Call(ctx, api.MethodAuthenticate, params, nil)
}

// SessionNew sends a session/new request to the client.
func (a *AgentConnection) SessionNew(
	ctx context.Context,
	params *api.NewSessionRequest,
) (*api.NewSessionResponse, error) {
	var result api.NewSessionResponse
	err := a.core.Call(ctx, api.MethodSessionNew, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionLoad sends a session/load request to the client.
func (a *AgentConnection) SessionLoad(ctx context.Context, params *api.LoadSessionRequest) error {
	return a.core.Call(ctx, api.MethodSessionLoad, params, nil)
}

// SessionPrompt sends a session/prompt request to the client.
func (a *AgentConnection) SessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	var result api.PromptResponse
	err := a.core.Call(ctx, api.MethodSessionPrompt, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionCancel sends a session/cancel notification to the client.
func (a *AgentConnection) SessionCancel(ctx context.Context, params *api.CancelNotification) error {
	return a.core.Notify(ctx, api.MethodSessionCancel, params)
}

// SessionRequestPermission sends a session/request_permission request to the client.
func (a *AgentConnection) SessionRequestPermission(
	ctx context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	var result api.RequestPermissionResponse
	err := a.core.Call(ctx, api.MethodSessionRequestPermission, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendSessionUpdate sends a session/update notification to the client.
func (a *AgentConnection) SendSessionUpdate(ctx context.Context, params *api.SessionNotification) error {
	return a.core.Notify(ctx, api.MethodSessionUpdate, params)
}
