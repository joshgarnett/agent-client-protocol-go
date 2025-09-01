package acp

import (
	"context"
	"errors"
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

// Errors for connection and notification handling.
var (
	ErrConnectionClosed       = errors.New("connection is closed")
	ErrNotificationBufferFull = errors.New("notification buffer is full")
)

const (
	// notificationChannelBufferSize is the buffer size for session update notifications.
	notificationChannelBufferSize = 50
)

// AgentConnection represents a connection from an agent to a client.
//
// This implementation uses a hybrid approach to prevent deadlocks:
//   - SendSessionUpdate uses a channel-based architecture (safe from handlers)
//   - Other methods like Call use direct JSON-RPC calls (can deadlock from handlers)
//
// This design follows the pattern used by the Rust reference implementation.
type AgentConnection struct {
	conn           *jsonrpc2.Conn
	handler        jsonrpc2.Handler
	broadcast      *StreamBroadcast
	state          *ConnectionStateTracker
	notificationCh chan *api.SessionNotification
	closeOnce      sync.Once
	closed         bool
	closedMu       sync.RWMutex
}

// NewAgentConnection creates a new agent connection with the given transport.
func NewAgentConnection(ctx context.Context, stream jsonrpc2.ObjectStream, handler jsonrpc2.Handler) *AgentConnection {
	conn := jsonrpc2.NewConn(ctx, stream, handler)
	broadcast := NewStreamBroadcast()

	ac := &AgentConnection{
		conn:           conn,
		handler:        handler,
		state:          NewConnectionStateTracker(),
		broadcast:      broadcast,
		notificationCh: make(chan *api.SessionNotification, notificationChannelBufferSize),
	}

	// Start the notification processing goroutine
	go ac.processNotifications(ctx)

	return ac
}

// NewAgentConnectionStdio creates a new agent connection using stdio transport.
func NewAgentConnectionStdio(ctx context.Context, rwc io.ReadWriteCloser, handler jsonrpc2.Handler) *AgentConnection {
	stream := jsonrpc2.NewPlainObjectStream(rwc)
	return NewAgentConnection(ctx, stream, handler)
}

// processNotifications processes session update notifications in a dedicated goroutine to avoid deadlocks.
// This matches the pattern used by the Rust reference implementation.
func (a *AgentConnection) processNotifications(ctx context.Context) {
	defer func() {
		a.markClosed()
	}()

	for {
		select {
		case notification, ok := <-a.notificationCh:
			if !ok {
				// Channel was closed, exit
				return
			}

			// Make the actual JSON-RPC notification call
			err := a.conn.Notify(ctx, api.MethodSessionUpdate, notification)
			if err != nil {
				// Log error but continue processing other notifications
				// In a production implementation, you might want more sophisticated error handling
				continue
			}

		case <-ctx.Done():
			return
		case <-a.conn.DisconnectNotify():
			return
		}
	}
}

// markClosed marks the connection as closed and closes the notification channel.
func (a *AgentConnection) markClosed() {
	a.closedMu.Lock()
	defer a.closedMu.Unlock()

	if a.closed {
		return
	}
	a.closed = true

	// Close the notification channel to signal no more messages
	close(a.notificationCh)
}

// isClosed returns true if the connection is closed.
func (a *AgentConnection) isClosed() bool {
	a.closedMu.RLock()
	defer a.closedMu.RUnlock()
	return a.closed
}

// call makes a JSON-RPC call to the client.
//
// WARNING: This method can deadlock if called from within a JSON-RPC handler
// on the same connection. Use SendSessionUpdate for notifications from handlers,
// or make calls from background goroutines for requests.
func (a *AgentConnection) call(ctx context.Context, method string, params, result any) error {
	return a.conn.Call(ctx, method, params, result)
}

// notify sends a JSON-RPC notification to the client using a channel-based approach
// to avoid deadlocks when called from within JSON-RPC handlers.
func (a *AgentConnection) notify(ctx context.Context, method string, params any) error {
	if a.isClosed() {
		return ErrConnectionClosed
	}

	// For session/update notifications, we need special handling with SessionNotification wrapper
	if method == api.MethodSessionUpdate {
		sessionNotification, ok := params.(*api.SessionNotification)
		if !ok {
			return errors.New("session/update method requires *api.SessionNotification parameter")
		}

		select {
		case a.notificationCh <- sessionNotification:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Channel is full - this indicates the notification processing is falling behind
			return ErrNotificationBufferFull
		}
	}

	// For other notifications, we still use the direct approach
	// TODO: Consider extending channel-based approach to all notifications if needed
	return a.conn.Notify(ctx, method, params)
}

// Close closes the connection.
func (a *AgentConnection) Close() error {
	var err error
	a.closeOnce.Do(func() {
		a.markClosed()
		if a.broadcast != nil {
			_ = a.broadcast.Close()
		}
		err = a.conn.Close()
	})
	return err
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
	err := a.call(ctx, api.MethodInitialize, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Authenticate sends an authenticate request to the client.
func (a *AgentConnection) Authenticate(ctx context.Context, params *api.AuthenticateRequest) error {
	return a.call(ctx, api.MethodAuthenticate, params, nil)
}

// SessionNew sends a session/new request to the client.
func (a *AgentConnection) SessionNew(
	ctx context.Context,
	params *api.NewSessionRequest,
) (*api.NewSessionResponse, error) {
	var result api.NewSessionResponse
	err := a.call(ctx, api.MethodSessionNew, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionLoad sends a session/load request to the client.
func (a *AgentConnection) SessionLoad(ctx context.Context, params *api.LoadSessionRequest) error {
	return a.call(ctx, api.MethodSessionLoad, params, nil)
}

// SessionPrompt sends a session/prompt request to the client.
func (a *AgentConnection) SessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	var result api.PromptResponse
	err := a.call(ctx, api.MethodSessionPrompt, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SessionCancel sends a session/cancel notification to the client.
func (a *AgentConnection) SessionCancel(ctx context.Context, params *api.CancelNotification) error {
	return a.notify(ctx, api.MethodSessionCancel, params)
}

// SessionRequestPermission sends a session/request_permission request to the client.
func (a *AgentConnection) SessionRequestPermission(
	ctx context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	var result api.RequestPermissionResponse
	err := a.call(ctx, api.MethodSessionRequestPermission, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendSessionUpdate sends a session/update notification to the client.
// This method is safe to call from within JSON-RPC handlers as it uses a channel-based
// approach to avoid deadlocks, following the pattern from the Rust reference implementation.
func (a *AgentConnection) SendSessionUpdate(ctx context.Context, params *api.SessionNotification) error {
	return a.notify(ctx, api.MethodSessionUpdate, params)
}

// Subscribe creates a new receiver for observing the message stream.
//
// This allows observing all JSON-RPC messages flowing through the connection
// for debugging, logging, or building development tools.
func (a *AgentConnection) Subscribe() *StreamReceiver {
	if a.broadcast == nil {
		// Return a receiver that's already closed if broadcast is not available
		ch := make(chan StreamMessage)
		close(ch)
		done := make(chan struct{})
		close(done)
		return &StreamReceiver{ch: ch, done: done}
	}
	return a.broadcast.Subscribe()
}
