package acp

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// StreamMessageDirection indicates the direction of a message relative to this side of the connection.
type StreamMessageDirection int

const (
	// StreamMessageDirectionIncoming indicates a message received from the other side of the connection.
	StreamMessageDirectionIncoming StreamMessageDirection = iota
	// StreamMessageDirectionOutgoing indicates a message sent to the other side of the connection.
	StreamMessageDirectionOutgoing
)

const (
	unknownDirection  = "unknown"
	defaultBufferSize = 64
)

// String returns the string representation of the direction.
func (d StreamMessageDirection) String() string {
	switch d {
	case StreamMessageDirectionIncoming:
		return "incoming"
	case StreamMessageDirectionOutgoing:
		return "outgoing"
	default:
		return unknownDirection
	}
}

// StreamMessageContent represents the content of a stream message.
// This enum represents the three types of JSON-RPC messages:
// - Requests: Method calls that expect a response
// - Responses: Replies to previous requests
// - Notifications: One-way messages that don't expect a response.
type StreamMessageContent interface {
	streamMessageContent()
}

// StreamMessageRequest represents a JSON-RPC request message.
type StreamMessageRequest struct {
	// The unique identifier for this request.
	ID int64 `json:"id"`
	// The name of the method being called.
	Method string `json:"method"`
	// Optional parameters for the method.
	Params json.RawMessage `json:"params,omitempty"`
}

func (StreamMessageRequest) streamMessageContent() {}

// StreamMessageResponse represents a JSON-RPC response message.
type StreamMessageResponse struct {
	// The ID of the request this response is for.
	ID int64 `json:"id"`
	// The result of the request (success or error).
	Result json.RawMessage `json:"result,omitempty"`
	Error  *StreamError    `json:"error,omitempty"`
}

func (StreamMessageResponse) streamMessageContent() {}

// StreamMessageNotification represents a JSON-RPC notification message.
type StreamMessageNotification struct {
	// The name of the notification method.
	Method string `json:"method"`
	// Optional parameters for the notification.
	Params json.RawMessage `json:"params,omitempty"`
}

func (StreamMessageNotification) streamMessageContent() {}

// StreamError represents an error in a stream message response.
type StreamError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *StreamError) Error() string {
	return e.Message
}

// StreamMessage represents a message that flows through the RPC stream.
//
// This represents any JSON-RPC message (request, response, or notification)
// along with its direction (incoming or outgoing).
//
// Stream messages are used for observing and debugging the protocol communication
// without interfering with the actual message handling.
type StreamMessage struct {
	// The direction of the message relative to this side of the connection.
	Direction StreamMessageDirection `json:"direction"`
	// The actual content of the message.
	Message StreamMessageContent `json:"message"`
	// Timestamp when the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// StreamReceiver allows receiving copies of all messages flowing through the connection.
//
// This allows you to receive copies of all messages flowing through the connection,
// useful for debugging, logging, or building development tools.
type StreamReceiver struct {
	ch   <-chan StreamMessage
	done <-chan struct{}
	once sync.Once
}

// Recv receives the next message from the stream.
//
// This method will wait until a message is available or the context is cancelled.
// Returns an error if the stream is closed or the context is cancelled.
func (sr *StreamReceiver) Recv(ctx context.Context) (*StreamMessage, error) {
	select {
	case msg, ok := <-sr.ch:
		if !ok {
			return nil, ErrStreamClosed
		}
		return &msg, nil
	case <-sr.done:
		return nil, ErrStreamClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the stream receiver.
func (sr *StreamReceiver) Close() error {
	sr.once.Do(func() {
		// The channel will be closed by the broadcaster
	})
	return nil
}

// StreamBroadcast manages broadcasting of stream messages to multiple receivers.
//
// This is used internally by the RPC connection to allow multiple receivers
// to observe the message stream.
type StreamBroadcast struct {
	mu        sync.RWMutex
	receivers []chan StreamMessage
	closed    bool
	done      chan struct{}
}

// NewStreamBroadcast creates a new stream broadcast.
func NewStreamBroadcast() *StreamBroadcast {
	return &StreamBroadcast{
		receivers: make([]chan StreamMessage, 0),
		done:      make(chan struct{}),
	}
}

// Subscribe creates a new receiver for observing the message stream.
//
// Each receiver will get its own copy of every message.
// The returned receiver should be closed when no longer needed to prevent memory leaks.
func (sb *StreamBroadcast) Subscribe() *StreamReceiver {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.closed {
		// Return a receiver that's already closed
		ch := make(chan StreamMessage)
		close(ch)
		return &StreamReceiver{ch: ch, done: sb.done}
	}

	// Create buffered channel to prevent blocking
	ch := make(chan StreamMessage, defaultBufferSize)
	sb.receivers = append(sb.receivers, ch)

	return &StreamReceiver{ch: ch, done: sb.done}
}

// Broadcast sends a message to all receivers.
//
// This method is non-blocking and will drop messages if receivers can't keep up.
func (sb *StreamBroadcast) Broadcast(message StreamMessage) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if sb.closed {
		return
	}

	// Send to all receivers, but don't block if they're full
	for i, ch := range sb.receivers {
		select {
		case ch <- message:
			// Message sent successfully
		default:
			// Receiver is full, skip it to prevent blocking
			// In a production system, we might want to close slow receivers
			_ = i // Prevent unused variable warning
		}
	}
}

// Close closes the stream broadcast and all associated receivers.
func (sb *StreamBroadcast) Close() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.closed {
		return nil
	}

	sb.closed = true
	close(sb.done)

	// Close all receiver channels
	for _, ch := range sb.receivers {
		close(ch)
	}
	sb.receivers = nil

	return nil
}

// ReceiverCount returns the number of active receivers.
func (sb *StreamBroadcast) ReceiverCount() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return len(sb.receivers)
}

// Common errors for stream operations.
var (
	ErrStreamClosed = &StreamError{
		Code:    -32001,
		Message: "Stream closed",
	}
)

// Helper functions for creating stream messages.

// NewStreamRequest creates a new stream message for a request.
func NewStreamRequest(direction StreamMessageDirection, id int64, method string, params json.RawMessage) StreamMessage {
	return StreamMessage{
		Direction: direction,
		Message: StreamMessageRequest{
			ID:     id,
			Method: method,
			Params: params,
		},
		Timestamp: time.Now(),
	}
}

// NewStreamResponse creates a new stream message for a response.
func NewStreamResponse(direction StreamMessageDirection, id int64,
	result json.RawMessage, err *StreamError) StreamMessage {
	return StreamMessage{
		Direction: direction,
		Message: StreamMessageResponse{
			ID:     id,
			Result: result,
			Error:  err,
		},
		Timestamp: time.Now(),
	}
}

// NewStreamNotification creates a new stream message for a notification.
func NewStreamNotification(direction StreamMessageDirection, method string, params json.RawMessage) StreamMessage {
	return StreamMessage{
		Direction: direction,
		Message: StreamMessageNotification{
			Method: method,
			Params: params,
		},
		Timestamp: time.Now(),
	}
}
