package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"golang.org/x/exp/jsonrpc2"
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
	ErrConnectionClosed = errors.New("connection is closed")
)

const (
	// testRequestTimeout is a shorter timeout used in tests.
	testRequestTimeout = 1 * time.Second
	// defaultCallQueueSize is the buffer size for the call queue.
	defaultCallQueueSize = 100
)

// queuedCall represents a call waiting to be processed.
type queuedCall struct {
	ctx        context.Context
	method     string
	params     interface{}
	resultChan chan callResult
}

// callResult holds the result of a queued call.
type callResult struct {
	result interface{}
	err    error
}

// AgentConnection represents a connection from an agent to a client.
// It abstracts away the underlying jsonrpc2 library and provides a safe way
// to make re-entrant calls from handlers.
type AgentConnection struct {
	conn           *jsonrpc2.Connection
	state          *ConnectionStateTracker
	requestTimeout time.Duration

	// Call queueing
	callQueue chan *queuedCall
	closeOnce sync.Once
	closed    chan struct{}
}

// binder is an internal struct that implements jsonrpc2.Binder to
// associate the user-provided handler with the connection.
type binder struct {
	handler Handler
}

// Bind is called by the jsonrpc2 library to bind the handler to the connection.
func (b *binder) Bind(_ context.Context, _ *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
	wrappedHandler := func(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
		// TODO: Pass actual connection reference instead of nil
		return b.handler.Handle(ctx, nil, req)
	}

	return jsonrpc2.ConnectionOptions{
		Handler: jsonrpc2.HandlerFunc(wrappedHandler),
	}, nil
}

// stdioDialer is a custom dialer that uses an existing io.ReadWriteCloser (like stdin/stdout).
type stdioDialer struct {
	rwc io.ReadWriteCloser
}

func (d stdioDialer) Dial(_ context.Context) (io.ReadWriteCloser, error) {
	return d.rwc, nil
}

// NewAgentConnectionStdio creates a new agent connection using stdio transport.
func NewAgentConnectionStdio(
	ctx context.Context,
	rwc io.ReadWriteCloser,
	handler Handler,
	timeout time.Duration,
) (*AgentConnection, error) {
	ac := &AgentConnection{
		state:          NewConnectionStateTracker(),
		requestTimeout: timeout,
		callQueue:      make(chan *queuedCall, defaultCallQueueSize),
		closed:         make(chan struct{}),
	}

	b := &binder{
		handler: handler,
	}

	// Create the connection using our custom dialer.
	conn, err := jsonrpc2.Dial(ctx, stdioDialer{rwc: rwc}, b)
	if err != nil {
		return nil, fmt.Errorf("failed to dial connection: %w", err)
	}

	ac.conn = conn

	// Start the call queue processor
	go ac.processCallQueue(ctx)

	return ac, nil
}

// processCallQueue processes queued calls sequentially to avoid writer contention.
func (a *AgentConnection) processCallQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.closed:
			return
		case queuedCall := <-a.callQueue:
			// Process the call directly on the connection
			result, err := a.callDirect(queuedCall.ctx, queuedCall.method, queuedCall.params)

			// Send result back to caller
			select {
			case queuedCall.resultChan <- callResult{result: result, err: err}:
			case <-queuedCall.ctx.Done():
				// Caller cancelled, ignore result
			}
		}
	}
}

// callDirect makes a direct JSON-RPC call without queueing.
func (a *AgentConnection) callDirect(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if a.conn == nil {
		return nil, ErrConnectionClosed
	}

	// Add a timeout to the context
	timeoutCtx, cancel := context.WithTimeout(ctx, a.requestTimeout)
	defer cancel()

	call := a.conn.Call(timeoutCtx, method, params)
	var result interface{}
	if err := call.Await(timeoutCtx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// call makes a JSON-RPC call to the client via the queue.
// This prevents writer contention in high concurrency scenarios.
func (a *AgentConnection) call(ctx context.Context, method string, params, result any) error {
	if a.conn == nil {
		return ErrConnectionClosed
	}

	// Create a queued call
	resultChan := make(chan callResult, 1)
	qCall := &queuedCall{
		ctx:        ctx,
		method:     method,
		params:     params,
		resultChan: resultChan,
	}

	// Queue the call
	select {
	case a.callQueue <- qCall:
	case <-ctx.Done():
		return ctx.Err()
	case <-a.closed:
		return ErrConnectionClosed
	}

	// Wait for result
	select {
	case res := <-resultChan:
		if res.err != nil {
			return res.err
		}
		// Unmarshal the result if needed
		if result != nil && res.result != nil {
			// Convert result via JSON marshaling
			jsonData, err := json.Marshal(res.result)
			if err != nil {
				return err
			}
			if unmarshalErr := json.Unmarshal(jsonData, result); unmarshalErr != nil {
				return unmarshalErr
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-a.closed:
		return ErrConnectionClosed
	}
}

// notify sends a JSON-RPC notification to the client.
func (a *AgentConnection) notify(ctx context.Context, method string, params any) error {
	if a.conn == nil {
		return ErrConnectionClosed
	}

	return a.conn.Notify(ctx, method, params)
}

// Close closes the connection.
func (a *AgentConnection) Close() error {
	if a.conn == nil {
		return ErrConnectionClosed
	}

	// Signal closure to queue processor
	a.closeOnce.Do(func() {
		close(a.closed)
	})

	return a.conn.Close()
}

// Wait waits for the connection to close.
func (a *AgentConnection) Wait() error {
	if a.conn == nil {
		return ErrConnectionClosed
	}
	return a.conn.Wait()
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
func (a *AgentConnection) SendSessionUpdate(ctx context.Context, params *api.SessionNotification) error {
	return a.notify(ctx, api.MethodSessionUpdate, params)
}

// Subscribe creates a new receiver for observing the message stream.
func (a *AgentConnection) Subscribe() *StreamReceiver {
	// Return a closed receiver - functionality not available in current implementation
	ch := make(chan StreamMessage)
	close(ch)
	done := make(chan struct{})
	close(done)
	return &StreamReceiver{ch: ch, done: done}
}
