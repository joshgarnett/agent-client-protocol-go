package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/util"
	"golang.org/x/exp/jsonrpc2"
)

const (
	// testRequestTimeout is a shorter timeout used in tests.
	testRequestTimeout = 1 * time.Second
	// defaultCallQueueSize is the buffer size for the call queue.
	defaultCallQueueSize = 100
)

// ConnectionState represents the current state of the protocol connection.
type ConnectionState int

const (
	StateUninitialized ConnectionState = iota
	StateInitialized
	StateAuthenticated
	StateSessionReady
)

// StateChangeCallback is called when connection state changes.
type StateChangeCallback func(from, to ConnectionState)

// stateTransitions defines valid state transitions.
var stateTransitions = map[ConnectionState][]ConnectionState{
	StateUninitialized: {StateInitialized},
	StateInitialized:   {StateAuthenticated, StateSessionReady},
	StateAuthenticated: {StateSessionReady},
	StateSessionReady:  {StateSessionReady}, // Can handle multiple sessions
}

// ConnectionCore handles the low-level jsonrpc2 connection and call queueing.
// This is shared between AgentConnection and ClientConnection to provide
// consistent behavior and prevent writer contention issues.
type ConnectionCore struct {
	conn           *jsonrpc2.Connection
	state          *util.AtomicValue[ConnectionState]
	stateCallbacks *util.CallbackRegistry[StateChangeCallback]
	requestTimeout time.Duration

	// Call queueing to prevent writer contention during re-entrant calls
	callQueue chan *queuedCall
	closeOnce sync.Once
	closed    chan struct{}
}

// NewConnectionCore creates a new connection core.
func NewConnectionCore(
	ctx context.Context,
	rwc io.ReadWriteCloser,
	handler Handler,
	timeout time.Duration,
) (*ConnectionCore, error) {
	core := &ConnectionCore{
		state:          util.NewAtomicValue(StateUninitialized),
		stateCallbacks: util.NewCallbackRegistry[StateChangeCallback](),
		requestTimeout: timeout,
		callQueue:      make(chan *queuedCall, defaultCallQueueSize),
		closed:         make(chan struct{}),
	}

	b := &binder{
		handler: handler,
		core:    core,
	}

	// Create the connection using our custom dialer.
	conn, err := jsonrpc2.Dial(ctx, stdioDialer{rwc: rwc}, b)
	if err != nil {
		return nil, fmt.Errorf("failed to dial connection: %w", err)
	}

	core.conn = conn

	// Start the call queue processor
	go core.processCallQueue(ctx)

	return core, nil
}

// processCallQueue processes queued calls sequentially to avoid writer contention.
func (c *ConnectionCore) processCallQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		case queuedCall := <-c.callQueue:
			// Process the call directly on the connection
			result, err := c.callDirect(queuedCall.ctx, queuedCall.method, queuedCall.params)

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
func (c *ConnectionCore) callDirect(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if c.conn == nil {
		return nil, ErrConnectionClosed
	}

	// Add a timeout to the context
	timeoutCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	call := c.conn.Call(timeoutCtx, method, params)
	var result interface{}
	if err := call.Await(timeoutCtx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Call makes a JSON-RPC call via the queue to prevent writer contention.
func (c *ConnectionCore) Call(ctx context.Context, method string, params, result any) error {
	if c.conn == nil {
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
	case c.callQueue <- qCall:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
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
	case <-c.closed:
		return ErrConnectionClosed
	}
}

// Notify sends a JSON-RPC notification.
func (c *ConnectionCore) Notify(ctx context.Context, method string, params any) error {
	if c.conn == nil {
		return ErrConnectionClosed
	}

	return c.conn.Notify(ctx, method, params)
}

// Close closes the connection.
func (c *ConnectionCore) Close() error {
	if c.conn == nil {
		return ErrConnectionClosed
	}

	// Signal closure to queue processor
	c.closeOnce.Do(func() {
		close(c.closed)
	})

	return c.conn.Close()
}

// Wait waits for the connection to close.
func (c *ConnectionCore) Wait() error {
	if c.conn == nil {
		return ErrConnectionClosed
	}
	return c.conn.Wait()
}

// getState returns the current connection state.
//
//nolint:unused // Reserved for internal state management
func (c *ConnectionCore) getState() ConnectionState {
	return c.state.Load()
}

// setState sets the connection state.
//
//nolint:unused // Reserved for internal state management
func (c *ConnectionCore) setState(newState ConnectionState) {
	oldState := c.state.Swap(newState)

	// Execute callbacks
	callbacks := c.stateCallbacks.GetAll()
	for _, callback := range callbacks {
		callback(oldState, newState)
	}
}

// OnStateChange registers a callback for state changes.
func (c *ConnectionCore) OnStateChange(callback StateChangeCallback) {
	c.stateCallbacks.Register(callback)
}

// canTransitionTo checks if a state transition is valid.
//
//nolint:unused // Reserved for internal state management
func (c *ConnectionCore) canTransitionTo(newState ConnectionState) bool {
	currentState := c.state.Load()
	validStates, exists := stateTransitions[currentState]
	if !exists {
		return false
	}

	for _, state := range validStates {
		if state == newState {
			return true
		}
	}
	return false
}

// transitionTo attempts to transition to a new state.
//
//nolint:unused // Reserved for internal state management
func (c *ConnectionCore) transitionTo(newState ConnectionState) error {
	if !c.canTransitionTo(newState) {
		currentState := c.state.Load()
		return fmt.Errorf("invalid state transition from %v to %v", currentState, newState)
	}

	c.setState(newState)
	return nil
}
