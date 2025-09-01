package acp

import (
	"context"
	"io"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// ClientConnection represents a connection from a client to an agent.
type ClientConnection struct {
	core *ConnectionCore
}

// NewClientConnectionStdio creates a new client connection using stdio transport.
func NewClientConnectionStdio(
	ctx context.Context,
	rwc io.ReadWriteCloser,
	handler Handler,
	timeout time.Duration,
) (*ClientConnection, error) {
	core, err := NewConnectionCore(ctx, rwc, handler, timeout)
	if err != nil {
		return nil, err
	}

	return &ClientConnection{core: core}, nil
}

// Close closes the connection.
func (c *ClientConnection) Close() error {
	return c.core.Close()
}

// Wait waits for the connection to close.
func (c *ClientConnection) Wait() error {
	return c.core.Wait()
}

// Client method helpers.

// FsReadTextFile sends a fs/read_text_file request to the agent.
func (c *ClientConnection) FsReadTextFile(
	ctx context.Context,
	params *api.ReadTextFileRequest,
) (*api.ReadTextFileResponse, error) {
	var result api.ReadTextFileResponse
	err := c.core.Call(ctx, api.MethodFsReadTextFile, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FsWriteTextFile sends a fs/write_text_file request to the agent.
func (c *ClientConnection) FsWriteTextFile(ctx context.Context, params *api.WriteTextFileRequest) error {
	return c.core.Call(ctx, api.MethodFsWriteTextFile, params, nil)
}

// SessionRequestPermission sends a session/request_permission request to the agent.
func (c *ClientConnection) SessionRequestPermission(
	ctx context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	var result api.RequestPermissionResponse
	err := c.core.Call(ctx, api.MethodSessionRequestPermission, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Initialize sends an initialize request to the agent.
func (c *ClientConnection) Initialize(
	ctx context.Context,
	params *api.InitializeRequest,
) (*api.InitializeResponse, error) {
	var result api.InitializeResponse
	err := c.core.Call(ctx, api.MethodInitialize, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionNew sends a session/new request to the agent.
func (c *ClientConnection) SessionNew(
	ctx context.Context,
	params *api.NewSessionRequest,
) (*api.NewSessionResponse, error) {
	var result api.NewSessionResponse
	err := c.core.Call(ctx, api.MethodSessionNew, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionPrompt sends a session/prompt request to the agent.
func (c *ClientConnection) SessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	var result api.PromptResponse
	err := c.core.Call(ctx, api.MethodSessionPrompt, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionCancel sends a session/cancel notification to the agent.
func (c *ClientConnection) SessionCancel(ctx context.Context, params *api.CancelNotification) error {
	return c.core.Notify(ctx, api.MethodSessionCancel, params)
}

// TerminalCreate sends a terminal/create request to the agent.
func (c *ClientConnection) TerminalCreate(
	ctx context.Context,
	params *api.CreateTerminalRequest,
) (*api.CreateTerminalResponse, error) {
	var result api.CreateTerminalResponse
	err := c.core.Call(ctx, api.MethodTerminalCreate, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalOutput sends a terminal/output notification to the agent.
func (c *ClientConnection) TerminalOutput(ctx context.Context, params *api.TerminalOutputRequest) error {
	return c.core.Notify(ctx, api.MethodTerminalOutput, params)
}

// TerminalRelease sends a terminal/release request to the agent.
func (c *ClientConnection) TerminalRelease(ctx context.Context, params *api.ReleaseTerminalRequest) error {
	return c.core.Call(ctx, api.MethodTerminalRelease, params, nil)
}

// TerminalKill sends a terminal/kill request to the agent.
func (c *ClientConnection) TerminalKill(ctx context.Context, params *api.KillTerminalRequest) error {
	return c.core.Call(ctx, api.MethodTerminalKill, params, nil)
}

// TerminalWaitForExit sends a terminal/wait_for_exit request to the agent.
func (c *ClientConnection) TerminalWaitForExit(
	ctx context.Context,
	params *api.WaitForTerminalExitRequest,
) (*api.WaitForTerminalExitResponse, error) {
	var result api.WaitForTerminalExitResponse
	err := c.core.Call(ctx, api.MethodTerminalWaitForExit, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
