package acp

import (
	"context"
	"io"
	"sync"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/sourcegraph/jsonrpc2"
)

// ConnectionState represents the current state of the protocol connection.
type ConnectionState int

const (
	StateUninitialized ConnectionState = iota
	StateInitialized
	StateAuthenticated
	StateSessionReady
)

// AgentConnection represents a connection from an agent to a client.
type AgentConnection struct {
	conn    *jsonrpc2.Conn
	handler jsonrpc2.Handler
	state   ConnectionState
	stateMu sync.RWMutex
}

// NewAgentConnection creates a new agent connection with the given transport.
func NewAgentConnection(ctx context.Context, stream jsonrpc2.ObjectStream, handler jsonrpc2.Handler) *AgentConnection {
	conn := jsonrpc2.NewConn(ctx, stream, handler)

	return &AgentConnection{
		conn:    conn,
		handler: handler,
		state:   StateUninitialized,
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
func (a *AgentConnection) Initialize(
	ctx context.Context,
	params *api.InitializeRequest,
) (*api.InitializeResponse, error) {
	var result api.InitializeResponse
	err := a.Call(ctx, api.MethodInitialize, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Authenticate sends an authenticate request to the client.
func (a *AgentConnection) Authenticate(ctx context.Context, params *api.AuthenticateRequest) error {
	return a.Call(ctx, api.MethodAuthenticate, params, nil)
}

// SessionNew sends a session/new request to the client.
func (a *AgentConnection) SessionNew(
	ctx context.Context,
	params *api.NewSessionRequest,
) (*api.NewSessionResponse, error) {
	var result api.NewSessionResponse
	err := a.Call(ctx, api.MethodSessionNew, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionLoad sends a session/load request to the client.
func (a *AgentConnection) SessionLoad(ctx context.Context, params *api.LoadSessionRequest) error {
	return a.Call(ctx, api.MethodSessionLoad, params, nil)
}

// SessionPrompt sends a session/prompt request to the client.
func (a *AgentConnection) SessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	var result api.PromptResponse
	err := a.Call(ctx, api.MethodSessionPrompt, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionCancel sends a session/cancel notification to the client.
func (a *AgentConnection) SessionCancel(ctx context.Context, params *api.CancelNotification) error {
	return a.Notify(ctx, api.MethodSessionCancel, params)
}

// SendSessionUpdate sends a session/update notification to the client.
func (a *AgentConnection) SendSessionUpdate(ctx context.Context, params *api.SessionNotification) error {
	return a.Notify(ctx, api.MethodSessionUpdate, params)
}

// GetState returns the current connection state.
func (a *AgentConnection) GetState() ConnectionState {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.state
}
