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

// Client method helpers - these are methods the Agent calls on the Client.

// FsReadTextFile sends a fs/read_text_file request to the client.
func (a *AgentConnection) FsReadTextFile(
	ctx context.Context,
	params *api.ReadTextFileRequest,
) (*api.ReadTextFileResponse, error) {
	var result api.ReadTextFileResponse
	err := a.core.Call(ctx, api.MethodFsReadTextFile, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FsWriteTextFile sends a fs/write_text_file request to the client.
func (a *AgentConnection) FsWriteTextFile(ctx context.Context, params *api.WriteTextFileRequest) error {
	return a.core.Call(ctx, api.MethodFsWriteTextFile, params, nil)
}

// Terminal method helpers - these are experimental/unstable methods the Agent calls on the Client.

// TerminalCreate sends a terminal/create request to the client.
func (a *AgentConnection) TerminalCreate(
	ctx context.Context,
	params *api.CreateTerminalRequest,
) (*api.CreateTerminalResponse, error) {
	var result api.CreateTerminalResponse
	err := a.core.Call(ctx, api.MethodTerminalCreate, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalOutput sends a terminal/output request to the client.
func (a *AgentConnection) TerminalOutput(
	ctx context.Context,
	params *api.TerminalOutputRequest,
) (*api.TerminalOutputResponse, error) {
	var result api.TerminalOutputResponse
	err := a.core.Call(ctx, api.MethodTerminalOutput, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalRelease sends a terminal/release request to the client.
func (a *AgentConnection) TerminalRelease(ctx context.Context, params *api.ReleaseTerminalRequest) error {
	return a.core.Call(ctx, api.MethodTerminalRelease, params, nil)
}

// TerminalWaitForExit sends a terminal/wait_for_exit request to the client.
func (a *AgentConnection) TerminalWaitForExit(
	ctx context.Context,
	params *api.WaitForTerminalExitRequest,
) (*api.WaitForTerminalExitResponse, error) {
	var result api.WaitForTerminalExitResponse
	err := a.core.Call(ctx, api.MethodTerminalWaitForExit, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalKill sends a terminal/kill request to the client.
func (a *AgentConnection) TerminalKill(ctx context.Context, params *api.KillTerminalRequest) error {
	return a.core.Call(ctx, api.MethodTerminalKill, params, nil)
}
