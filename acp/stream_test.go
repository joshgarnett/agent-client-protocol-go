package acp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamMessageDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction StreamMessageDirection
		expected  string
	}{
		{"Incoming", StreamMessageDirectionIncoming, "incoming"},
		{"Outgoing", StreamMessageDirectionOutgoing, "outgoing"},
		{"Unknown", StreamMessageDirection(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.direction.String())
		})
	}
}

func TestStreamBroadcast(t *testing.T) {
	t.Run("Subscribe and Broadcast", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		// Subscribe multiple receivers
		receiver1 := broadcast.Subscribe()
		receiver2 := broadcast.Subscribe()

		// Send a message
		msg := NewStreamRequest(StreamMessageDirectionOutgoing, 1, "test.method", json.RawMessage(`{"test": true}`))
		broadcast.Broadcast(msg)

		// Both receivers should get the message
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		received1, err := receiver1.Recv(ctx)
		require.NoError(t, err)
		assert.Equal(t, StreamMessageDirectionOutgoing, received1.Direction)

		received2, err := receiver2.Recv(ctx)
		require.NoError(t, err)
		assert.Equal(t, StreamMessageDirectionOutgoing, received2.Direction)
	})

	t.Run("Multiple Messages", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		receiver := broadcast.Subscribe()

		// Send multiple messages
		for i := range 5 {
			msg := NewStreamRequest(StreamMessageDirectionOutgoing, int64(i), "test.method", nil)
			broadcast.Broadcast(msg)
		}

		// Receive all messages
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		for range 5 {
			msg, err := receiver.Recv(ctx)
			require.NoError(t, err)
			require.NotNil(t, msg)
		}
	})

	t.Run("Close Broadcast", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		receiver := broadcast.Subscribe()

		// Close the broadcast
		err := broadcast.Close()
		require.NoError(t, err)

		// Receiver should get error on receive
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = receiver.Recv(ctx)
		assert.Error(t, err)
	})

	t.Run("Concurrent Broadcasting", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		receiver := broadcast.Subscribe()

		// Send messages concurrently
		var wg sync.WaitGroup
		messageCount := 100

		for i := range messageCount {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				msg := NewStreamRequest(StreamMessageDirectionOutgoing, id, "test.method", nil)
				broadcast.Broadcast(msg)
			}(int64(i))
		}

		wg.Wait()

		// Receive all messages
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		received := 0
		for {
			select {
			case <-ctx.Done():
				goto done
			default:
				msg, err := receiver.Recv(ctx)
				if err == nil && msg != nil {
					received++
					if received == messageCount {
						goto done
					}
				}
			}
		}
	done:
		// Should receive at least the buffer size (64) but may not get all 100
		// due to non-blocking sends to prevent deadlock
		assert.GreaterOrEqual(t, received, 64)
	})

	t.Run("ReceiverCount", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		assert.Equal(t, 0, broadcast.ReceiverCount())

		r1 := broadcast.Subscribe()
		assert.Equal(t, 1, broadcast.ReceiverCount())

		r2 := broadcast.Subscribe()
		assert.Equal(t, 2, broadcast.ReceiverCount())

		// Closing receivers doesn't affect count immediately
		r1.Close()
		r2.Close()
		assert.Equal(t, 2, broadcast.ReceiverCount())
	})

	t.Run("Subscribe After Close", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		broadcast.Close()

		// Subscribe after close should return closed receiver
		receiver := broadcast.Subscribe()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := receiver.Recv(ctx)
		assert.Error(t, err)
	})
}

func TestStreamReceiver(t *testing.T) {
	t.Run("Recv Timeout", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		receiver := broadcast.Subscribe()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Should timeout waiting for message
		_, err := receiver.Recv(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("Recv Cancellation", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		receiver := broadcast.Subscribe()

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context immediately
		cancel()

		_, err := receiver.Recv(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Close Receiver", func(t *testing.T) {
		broadcast := NewStreamBroadcast()
		defer broadcast.Close()

		receiver := broadcast.Subscribe()

		// Close receiver
		err := receiver.Close()
		require.NoError(t, err)

		// Close again should be idempotent
		err = receiver.Close()
		assert.NoError(t, err)
	})
}

func TestStreamMessageHelpers(t *testing.T) {
	t.Run("NewStreamRequest", func(t *testing.T) {
		params := json.RawMessage(`{"test": true}`)
		msg := NewStreamRequest(StreamMessageDirectionIncoming, 123, "test.method", params)

		assert.Equal(t, StreamMessageDirectionIncoming, msg.Direction)
		assert.NotZero(t, msg.Timestamp)

		req, ok := msg.Message.(StreamMessageRequest)
		require.True(t, ok)
		assert.Equal(t, int64(123), req.ID)
		assert.Equal(t, "test.method", req.Method)
		assert.Equal(t, params, req.Params)
	})

	t.Run("NewStreamResponse", func(t *testing.T) {
		result := json.RawMessage(`{"success": true}`)
		msg := NewStreamResponse(StreamMessageDirectionIncoming, 456, result, nil)

		assert.Equal(t, StreamMessageDirectionIncoming, msg.Direction)
		assert.NotZero(t, msg.Timestamp)

		resp, ok := msg.Message.(StreamMessageResponse)
		require.True(t, ok)
		assert.Equal(t, int64(456), resp.ID)
		assert.Equal(t, result, resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("NewStreamResponse with Error", func(t *testing.T) {
		streamErr := &StreamError{
			Code:    -32603,
			Message: "Internal error",
		}
		msg := NewStreamResponse(StreamMessageDirectionIncoming, 789, nil, streamErr)

		resp, ok := msg.Message.(StreamMessageResponse)
		require.True(t, ok)
		assert.Equal(t, int64(789), resp.ID)
		assert.Nil(t, resp.Result)
		assert.Equal(t, streamErr, resp.Error)
	})

	t.Run("NewStreamNotification", func(t *testing.T) {
		params := json.RawMessage(`{"event": "update"}`)
		msg := NewStreamNotification(StreamMessageDirectionOutgoing, "notify.event", params)

		assert.Equal(t, StreamMessageDirectionOutgoing, msg.Direction)
		assert.NotZero(t, msg.Timestamp)

		notif, ok := msg.Message.(StreamMessageNotification)
		require.True(t, ok)
		assert.Equal(t, "notify.event", notif.Method)
		assert.Equal(t, params, notif.Params)
	})
}

func TestStreamError(t *testing.T) {
	t.Run("Error Interface", func(t *testing.T) {
		err := &StreamError{
			Code:    -32000,
			Message: "Test error",
		}

		assert.Equal(t, "Test error", err.Error())
	})

	t.Run("ErrStreamClosed", func(t *testing.T) {
		assert.Equal(t, -32001, ErrStreamClosed.Code)
		assert.Equal(t, "Stream closed", ErrStreamClosed.Message)
		assert.Equal(t, "Stream closed", ErrStreamClosed.Error())
	})
}

func TestStreamBroadcastBuffering(t *testing.T) {
	broadcast := NewStreamBroadcast()
	defer broadcast.Close()

	receiver := broadcast.Subscribe()

	// Send more messages than buffer size to test non-blocking
	for i := range 100 {
		msg := NewStreamRequest(StreamMessageDirectionOutgoing, int64(i), "test", nil)
		broadcast.Broadcast(msg) // Should not block even if receiver is slow
	}

	// Now receive messages
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	received := 0
	for {
		select {
		case <-ctx.Done():
			t.Logf("Received %d messages before timeout", received)
			return
		default:
			_, err := receiver.Recv(ctx)
			if err == nil {
				received++
				if received >= 64 { // At least buffer size
					return
				}
			}
		}
	}
}
