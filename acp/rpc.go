package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/jsonrpc2"
)

const defaultTimeoutSecs = 30

// RPCConnection provides enhanced RPC functionality on top of the basic jsonrpc2 connection.
//
// It adds features like:
// - Request/response tracking with atomic ID generation
// - Timeout handling per request
// - Message broadcasting for stream observation
// - Concurrent request management.
type RPCConnection struct {
	// Underlying jsonrpc2 connection
	conn *jsonrpc2.Conn

	// Request/response tracking
	pendingRequests map[interface{}]chan *ResponseResult
	pendingMu       sync.RWMutex
	nextID          int64

	// Stream broadcasting
	broadcast *StreamBroadcast

	// Configuration
	defaultTimeout time.Duration

	// Cleanup
	closed bool
	mu     sync.RWMutex
}

// ResponseResult holds the result of an RPC request.
type ResponseResult struct {
	Result json.RawMessage
	Error  *jsonrpc2.Error
}

// RPCConnectionConfig configures an RPCConnection.
type RPCConnectionConfig struct {
	// DefaultTimeout for requests (default: 30 seconds)
	DefaultTimeout time.Duration
	// EnableStreaming enables message broadcasting (default: true)
	EnableStreaming bool
}

// NewRPCConnection creates a new enhanced RPC connection wrapping the given jsonrpc2 connection.
func NewRPCConnection(conn *jsonrpc2.Conn, config *RPCConnectionConfig) *RPCConnection {
	if config == nil {
		config = &RPCConnectionConfig{
			DefaultTimeout:  defaultTimeoutSecs * time.Second,
			EnableStreaming: true,
		}
	}

	var broadcast *StreamBroadcast
	if config.EnableStreaming {
		broadcast = NewStreamBroadcast()
	}

	rpc := &RPCConnection{
		conn:            conn,
		pendingRequests: make(map[interface{}]chan *ResponseResult),
		broadcast:       broadcast,
		defaultTimeout:  config.DefaultTimeout,
	}

	return rpc
}

// Call makes an RPC call with optional timeout.
func (r *RPCConnection) Call(ctx context.Context, method string, params, result interface{}) error {
	return r.CallWithTimeout(ctx, method, params, result, r.defaultTimeout)
}

// CallWithTimeout makes an RPC call with a specific timeout.
func (r *RPCConnection) CallWithTimeout(ctx context.Context, method string, params,
	result interface{}, timeout time.Duration) error {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return errors.New("connection is closed")
	}
	r.mu.RUnlock()

	// Generate unique request ID
	id := atomic.AddInt64(&r.nextID, 1)

	// Serialize parameters for broadcasting
	var paramsRaw json.RawMessage
	if params != nil {
		paramBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsRaw = paramBytes
	}

	// Broadcast outgoing request
	if r.broadcast != nil {
		msg := NewStreamRequest(StreamMessageDirectionOutgoing, id, method, paramsRaw)
		r.broadcast.Broadcast(msg)
	}

	// Create timeout context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Set up response channel
	respCh := make(chan *ResponseResult, 1)
	r.pendingMu.Lock()
	r.pendingRequests[id] = respCh
	r.pendingMu.Unlock()

	// Cleanup on return
	defer func() {
		r.pendingMu.Lock()
		delete(r.pendingRequests, id)
		close(respCh)
		r.pendingMu.Unlock()
	}()

	// Make the actual call
	err := r.conn.Call(ctx, method, params, result)

	// Broadcast response (success or error)
	r.broadcastResponse(id, result, err)

	return err
}

// broadcastResponse broadcasts a response message if broadcasting is enabled.
func (r *RPCConnection) broadcastResponse(id int64, result interface{}, err error) {
	if r.broadcast == nil {
		return
	}

	var resultRaw json.RawMessage
	var streamErr *StreamError

	if err != nil {
		streamErr = r.convertToStreamError(err)
	} else if result != nil {
		if resultBytes, marshalErr := json.Marshal(result); marshalErr == nil {
			resultRaw = resultBytes
		}
	}

	msg := NewStreamResponse(StreamMessageDirectionIncoming, id, resultRaw, streamErr)
	r.broadcast.Broadcast(msg)
}

// convertToStreamError converts an error to a StreamError.
func (r *RPCConnection) convertToStreamError(err error) *StreamError {
	var rpcErr *jsonrpc2.Error
	if errors.As(err, &rpcErr) {
		streamErr := &StreamError{
			Code:    int(rpcErr.Code),
			Message: rpcErr.Message,
		}
		if rpcErr.Data != nil {
			if data, marshalErr := json.Marshal(rpcErr.Data); marshalErr == nil {
				streamErr.Data = data
			}
		}
		return streamErr
	}

	return &StreamError{
		Code:    -32603,
		Message: err.Error(),
	}
}

// Notify sends an RPC notification.
func (r *RPCConnection) Notify(ctx context.Context, method string, params interface{}) error {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return errors.New("connection is closed")
	}
	r.mu.RUnlock()

	// Serialize parameters for broadcasting
	var paramsRaw json.RawMessage
	if params != nil {
		paramBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsRaw = paramBytes
	}

	// Broadcast outgoing notification
	if r.broadcast != nil {
		msg := NewStreamNotification(StreamMessageDirectionOutgoing, method, paramsRaw)
		r.broadcast.Broadcast(msg)
	}

	return r.conn.Notify(ctx, method, params)
}

// Subscribe creates a receiver for observing message stream.
func (r *RPCConnection) Subscribe() *StreamReceiver {
	if r.broadcast == nil {
		// Return closed receiver if streaming is disabled
		ch := make(chan StreamMessage)
		close(ch)
		done := make(chan struct{})
		close(done)
		return &StreamReceiver{ch: ch, done: done}
	}
	return r.broadcast.Subscribe()
}

// Close closes the RPC connection.
func (r *RPCConnection) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	// Cancel all pending requests
	r.pendingMu.Lock()
	for _, ch := range r.pendingRequests {
		select {
		case ch <- &ResponseResult{Error: &jsonrpc2.Error{Code: -32000, Message: "Connection closed"}}:
		default:
		}
	}
	r.pendingRequests = make(map[interface{}]chan *ResponseResult)
	r.pendingMu.Unlock()

	// Close broadcast
	if r.broadcast != nil {
		_ = r.broadcast.Close()
	}

	return r.conn.Close()
}

// Wait waits for the underlying connection to close.
func (r *RPCConnection) Wait() error {
	<-r.conn.DisconnectNotify()
	return nil
}

// Underlying returns the underlying jsonrpc2 connection.
// This should be used with caution as it bypasses the enhanced features.
func (r *RPCConnection) Underlying() *jsonrpc2.Conn {
	return r.conn
}

// PendingRequestCount returns the number of pending requests.
func (r *RPCConnection) PendingRequestCount() int {
	r.pendingMu.RLock()
	defer r.pendingMu.RUnlock()
	return len(r.pendingRequests)
}

// SetDefaultTimeout sets the default timeout for requests.
func (r *RPCConnection) SetDefaultTimeout(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultTimeout = timeout
}

// GetDefaultTimeout returns the current default timeout.
func (r *RPCConnection) GetDefaultTimeout() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultTimeout
}

// InterceptedHandler wraps a jsonrpc2 handler to intercept messages for stream broadcasting.
type InterceptedHandler struct {
	underlying jsonrpc2.Handler
	broadcast  *StreamBroadcast
}

// NewInterceptedHandler creates a handler that intercepts messages for broadcasting.
func NewInterceptedHandler(handler jsonrpc2.Handler, broadcast *StreamBroadcast) *InterceptedHandler {
	return &InterceptedHandler{
		underlying: handler,
		broadcast:  broadcast,
	}
}

// Handle implements jsonrpc2.Handler with message interception.
func (h *InterceptedHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// Broadcast incoming message
	if h.broadcast != nil {
		var paramsRaw json.RawMessage
		if req.Params != nil {
			paramsRaw = *req.Params
		}

		var msg StreamMessage
		if req.Notif {
			// It's a notification
			msg = NewStreamNotification(StreamMessageDirectionIncoming, req.Method, paramsRaw)
		} else {
			// It's a request
			// Simple approach: use hash of method name if ID is not accessible
			// This is for broadcasting purposes only, so exact ID matching isn't critical
			id := int64(len(req.Method))
			msg = NewStreamRequest(StreamMessageDirectionIncoming, id, req.Method, paramsRaw)
		}

		h.broadcast.Broadcast(msg)
	}

	// Call the underlying handler
	h.underlying.Handle(ctx, conn, req)
}
