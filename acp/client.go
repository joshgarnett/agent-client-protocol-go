package acp

import (
	"context"
	"io"

	"github.com/sourcegraph/jsonrpc2"
)

// ClientConnection represents a connection from a client to an agent.
type ClientConnection struct {
	conn    *jsonrpc2.Conn
	handler jsonrpc2.Handler
}

// NewClientConnection creates a new client connection with the given transport.
func NewClientConnection(
	ctx context.Context,
	stream jsonrpc2.ObjectStream,
	handler jsonrpc2.Handler,
) *ClientConnection {
	conn := jsonrpc2.NewConn(ctx, stream, handler)

	return &ClientConnection{
		conn:    conn,
		handler: handler,
	}
}

// NewClientConnectionStdio creates a new client connection using stdio transport.
func NewClientConnectionStdio(ctx context.Context, rwc io.ReadWriteCloser, handler jsonrpc2.Handler) *ClientConnection {
	stream := jsonrpc2.NewPlainObjectStream(rwc)
	return NewClientConnection(ctx, stream, handler)
}

// Call makes a JSON-RPC call to the agent.
func (c *ClientConnection) Call(ctx context.Context, method string, params, result any) error {
	return c.conn.Call(ctx, method, params, result)
}

// Notify sends a JSON-RPC notification to the agent.
func (c *ClientConnection) Notify(ctx context.Context, method string, params any) error {
	return c.conn.Notify(ctx, method, params)
}

// Close closes the connection.
func (c *ClientConnection) Close() error {
	return c.conn.Close()
}

// Wait waits for the connection to close.
func (c *ClientConnection) Wait() error {
	<-c.conn.DisconnectNotify()
	return nil
}

// Client method helpers.

// FsReadTextFile sends a fs/read_text_file request to the agent.
func (c *ClientConnection) FsReadTextFile(
	ctx context.Context,
	params *ReadTextFileRequest,
) (*ReadTextFileResponse, error) {
	var result ReadTextFileResponse
	err := c.Call(ctx, MethodFsReadTextFile, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FsWriteTextFile sends a fs/write_text_file request to the agent.
func (c *ClientConnection) FsWriteTextFile(ctx context.Context, params *WriteTextFileRequest) error {
	return c.Call(ctx, MethodFsWriteTextFile, params, nil)
}

// SessionRequestPermission sends a session/request_permission request to the agent.
func (c *ClientConnection) SessionRequestPermission(
	ctx context.Context,
	params *RequestPermissionRequest,
) (*RequestPermissionResponse, error) {
	var result RequestPermissionResponse
	err := c.Call(ctx, MethodSessionRequestPermission, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalCreate sends a terminal/create request to the agent.
func (c *ClientConnection) TerminalCreate(
	ctx context.Context,
	params *CreateTerminalRequest,
) (*CreateTerminalResponse, error) {
	var result CreateTerminalResponse
	err := c.Call(ctx, MethodTerminalCreate, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalOutput sends a terminal/output notification to the agent.
func (c *ClientConnection) TerminalOutput(ctx context.Context, params *TerminalOutputRequest) error {
	return c.Notify(ctx, MethodTerminalOutput, params)
}

// TerminalRelease sends a terminal/release request to the agent.
func (c *ClientConnection) TerminalRelease(ctx context.Context, params *ReleaseTerminalRequest) error {
	return c.Call(ctx, MethodTerminalRelease, params, nil)
}

// TerminalWaitForExit sends a terminal/wait_for_exit request to the agent.
func (c *ClientConnection) TerminalWaitForExit(
	ctx context.Context,
	params *WaitForTerminalExitRequest,
) (*WaitForTerminalExitResponse, error) {
	var result WaitForTerminalExitResponse
	err := c.Call(ctx, MethodTerminalWaitForExit, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
