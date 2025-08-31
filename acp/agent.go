package acp

import (
	"context"
	"io"

	"github.com/sourcegraph/jsonrpc2"
)

// AgentConnection represents a connection from an agent to a client.
type AgentConnection struct {
	conn    *jsonrpc2.Conn
	handler jsonrpc2.Handler
}

// NewAgentConnection creates a new agent connection with the given transport.
func NewAgentConnection(ctx context.Context, stream jsonrpc2.ObjectStream, handler jsonrpc2.Handler) *AgentConnection {
	conn := jsonrpc2.NewConn(ctx, stream, handler)

	return &AgentConnection{
		conn:    conn,
		handler: handler,
	}
}

// NewAgentConnectionStdio creates a new agent connection using stdio transport.
func NewAgentConnectionStdio(ctx context.Context, rwc io.ReadWriteCloser, handler jsonrpc2.Handler) *AgentConnection {
	stream := jsonrpc2.NewPlainObjectStream(rwc)
	return NewAgentConnection(ctx, stream, handler)
}

// Call makes a JSON-RPC call to the client.
func (a *AgentConnection) Call(ctx context.Context, method string, params, result any) error {
	return a.conn.Call(ctx, method, params, result)
}

// Notify sends a JSON-RPC notification to the client.
func (a *AgentConnection) Notify(ctx context.Context, method string, params any) error {
	return a.conn.Notify(ctx, method, params)
}

// Close closes the connection.
func (a *AgentConnection) Close() error {
	return a.conn.Close()
}

// Wait waits for the connection to close.
func (a *AgentConnection) Wait() error {
	<-a.conn.DisconnectNotify()
	return nil
}

// Agent method helpers.

// Initialize sends an initialize request to the client.
func (a *AgentConnection) Initialize(ctx context.Context, params *InitializeRequest) (*InitializeResponse, error) {
	var result InitializeResponse
	err := a.Call(ctx, MethodInitialize, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Authenticate sends an authenticate request to the client.
func (a *AgentConnection) Authenticate(ctx context.Context, params *AuthenticateRequest) error {
	return a.Call(ctx, MethodAuthenticate, params, nil)
}

// SessionNew sends a session/new request to the client.
func (a *AgentConnection) SessionNew(ctx context.Context, params *NewSessionRequest) (*NewSessionResponse, error) {
	var result NewSessionResponse
	err := a.Call(ctx, MethodSessionNew, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionLoad sends a session/load request to the client.
func (a *AgentConnection) SessionLoad(ctx context.Context, params *LoadSessionRequest) error {
	return a.Call(ctx, MethodSessionLoad, params, nil)
}

// SessionPrompt sends a session/prompt request to the client.
func (a *AgentConnection) SessionPrompt(ctx context.Context, params *PromptRequest) (*PromptResponse, error) {
	var result PromptResponse
	err := a.Call(ctx, MethodSessionPrompt, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionCancel sends a session/cancel notification to the client.
func (a *AgentConnection) SessionCancel(ctx context.Context, params *CancelNotification) error {
	return a.Notify(ctx, MethodSessionCancel, params)
}
