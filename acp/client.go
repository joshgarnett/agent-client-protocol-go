package acp

import (
	"context"
	"io"
	"sync"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/sourcegraph/jsonrpc2"
)

// ClientConnection represents a connection from a client to an agent.
type ClientConnection struct {
	conn                 *jsonrpc2.Conn
	handler              jsonrpc2.Handler
	broadcast            *StreamBroadcast
	state                ConnectionState
	stateChangeCallbacks []StateChangeCallback
	mu                   sync.RWMutex
}

// NewClientConnection creates a new client connection with the given transport.
func NewClientConnection(
	ctx context.Context,
	stream jsonrpc2.ObjectStream,
	handler jsonrpc2.Handler,
) *ClientConnection {
	conn := jsonrpc2.NewConn(ctx, stream, handler)
	broadcast := NewStreamBroadcast()

	return &ClientConnection{
		conn:      conn,
		handler:   handler,
		broadcast: broadcast,
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
	if c.broadcast != nil {
		_ = c.broadcast.Close()
	}
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
	params *api.ReadTextFileRequest,
) (*api.ReadTextFileResponse, error) {
	var result api.ReadTextFileResponse
	err := c.Call(ctx, api.MethodFsReadTextFile, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FsWriteTextFile sends a fs/write_text_file request to the agent.
func (c *ClientConnection) FsWriteTextFile(ctx context.Context, params *api.WriteTextFileRequest) error {
	return c.Call(ctx, api.MethodFsWriteTextFile, params, nil)
}

// SessionRequestPermission sends a session/request_permission request to the agent.
func (c *ClientConnection) SessionRequestPermission(
	ctx context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	var result api.RequestPermissionResponse
	err := c.Call(ctx, api.MethodSessionRequestPermission, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalCreate sends a terminal/create request to the agent.
func (c *ClientConnection) TerminalCreate(
	ctx context.Context,
	params *api.CreateTerminalRequest,
) (*api.CreateTerminalResponse, error) {
	var result api.CreateTerminalResponse
	err := c.Call(ctx, api.MethodTerminalCreate, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TerminalOutput sends a terminal/output notification to the agent.
func (c *ClientConnection) TerminalOutput(ctx context.Context, params *api.TerminalOutputRequest) error {
	return c.Notify(ctx, api.MethodTerminalOutput, params)
}

// TerminalRelease sends a terminal/release request to the agent.
func (c *ClientConnection) TerminalRelease(ctx context.Context, params *api.ReleaseTerminalRequest) error {
	return c.Call(ctx, api.MethodTerminalRelease, params, nil)
}

// TerminalWaitForExit sends a terminal/wait_for_exit request to the agent.
func (c *ClientConnection) TerminalWaitForExit(
	ctx context.Context,
	params *api.WaitForTerminalExitRequest,
) (*api.WaitForTerminalExitResponse, error) {
	var result api.WaitForTerminalExitResponse
	err := c.Call(ctx, api.MethodTerminalWaitForExit, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Subscribe creates a new receiver for observing the message stream.
//
// This allows observing all JSON-RPC messages flowing through the connection
// for debugging, logging, or building development tools.
func (c *ClientConnection) Subscribe() *StreamReceiver {
	if c.broadcast == nil {
		// Return a receiver that's already closed if broadcast is not available
		ch := make(chan StreamMessage)
		close(ch)
		done := make(chan struct{})
		close(done)
		return &StreamReceiver{ch: ch, done: done}
	}
	return c.broadcast.Subscribe()
}
