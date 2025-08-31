package acp

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/suite"
)

// TransportTestSuite tests the transport layer functionality.
type TransportTestSuite struct {
	suite.Suite

	transport *MockTransport
}

func (s *TransportTestSuite) SetupTest() {
	s.transport = NewMockTransport()
}

func (s *TransportTestSuite) TearDownTest() {
	if s.transport != nil {
		s.transport.Close()
	}
}

func (s *TransportTestSuite) TestMockTransportBidirectional() {
	// Get both streams.
	clientStream := s.transport.ClientStream()
	agentStream := s.transport.AgentStream()

	// Test client to agent communication.
	testObj := map[string]interface{}{
		"method": "test",
		"params": []string{"hello", "world"},
	}

	// Client writes, agent should read.
	err := clientStream.WriteObject(testObj)
	s.Require().NoError(err)

	var received map[string]interface{}
	err = agentStream.ReadObject(&received)
	s.Require().NoError(err)
	s.Equal("test", received["method"])
	// JSON unmarshaling converts []string to []interface{}.
	params, ok := received["params"].([]interface{})
	s.True(ok)
	s.Len(params, 2)
	s.Equal("hello", params[0])
	s.Equal("world", params[1])

	// Test agent to client communication.
	responseObj := map[string]interface{}{
		"result": "success",
		"id":     123,
	}

	// Agent writes, client should read.
	err = agentStream.WriteObject(responseObj)
	s.Require().NoError(err)

	var responseReceived map[string]interface{}
	err = clientStream.ReadObject(&responseReceived)
	s.Require().NoError(err)
	s.Equal("success", responseReceived["result"])
	// JSON unmarshaling converts int to float64.
	s.InDelta(float64(123), responseReceived["id"], 0.01)
}

func (s *TransportTestSuite) TestMockTransportClosure() {
	clientStream := s.transport.ClientStream()
	agentStream := s.transport.AgentStream()

	// Close transport.
	err := s.transport.Close()
	s.Require().NoError(err)

	// Reads should return EOF after close.
	var obj map[string]interface{}
	err = clientStream.ReadObject(&obj)
	s.Equal(io.EOF, err)

	err = agentStream.ReadObject(&obj)
	s.Equal(io.EOF, err)

	// Writes should fail after close.
	err = clientStream.WriteObject(map[string]string{"test": "value"})
	s.Equal(io.ErrClosedPipe, err)

	err = agentStream.WriteObject(map[string]string{"test": "value"})
	s.Equal(io.ErrClosedPipe, err)
}

func (s *TransportTestSuite) TestMockTransportConcurrentAccess() {
	clientStream := s.transport.ClientStream()
	agentStream := s.transport.AgentStream()

	const numMessages = 100
	var wg sync.WaitGroup

	// Channel to collect received messages.
	received := make(chan map[string]interface{}, numMessages)

	// Start reader goroutine (agent reading from client).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range numMessages {
			var obj map[string]interface{}
			err := agentStream.ReadObject(&obj)
			if err != nil {
				s.T().Errorf("Read error: %v", err)
				return
			}
			received <- obj
		}
	}()

	// Start writer goroutines (client writing to agent).
	numWriters := 10
	messagesPerWriter := numMessages / numWriters

	for w := range numWriters {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := range messagesPerWriter {
				obj := map[string]interface{}{
					"writer":  writerID,
					"message": i,
					"data":    []string{"concurrent", "test"},
				}
				err := clientStream.WriteObject(obj)
				if err != nil {
					s.T().Errorf("Write error: %v", err)
					return
				}
			}
		}(w)
	}

	// Wait for all operations to complete.
	wg.Wait()
	close(received)

	// Verify all messages were received.
	receivedCount := 0
	receivedMap := make(map[string]map[int]bool)

	for msg := range received {
		receivedCount++
		writer := int(msg["writer"].(float64))
		message := int(msg["message"].(float64))

		if receivedMap[string(rune(writer))] == nil {
			receivedMap[string(rune(writer))] = make(map[int]bool)
		}
		receivedMap[string(rune(writer))][message] = true
	}

	s.Equal(numMessages, receivedCount)
}

func (s *TransportTestSuite) TestConnectionLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create handler that counts method calls.
	callCount := 0
	var mu sync.Mutex
	handler := jsonrpc2.HandlerWithError(
		func(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) (interface{}, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return map[string]string{"response": "ok"}, nil
		},
	)

	// Create connections.
	clientConn := jsonrpc2.NewConn(ctx, s.transport.ClientStream(), handler)
	agentConn := jsonrpc2.NewConn(ctx, s.transport.AgentStream(), handler)

	// Test method calls.
	var result map[string]interface{}
	err := clientConn.Call(ctx, "testMethod", map[string]string{"param": "value"}, &result)
	s.Require().NoError(err)
	s.Equal("ok", result["response"])

	// Close connections.
	clientConn.Close()
	agentConn.Close()

	// Wait for connections to close.
	<-clientConn.DisconnectNotify()
	<-agentConn.DisconnectNotify()

	// Verify handler was called.
	mu.Lock()
	s.Equal(1, callCount)
	mu.Unlock()
}

func (s *TransportTestSuite) TestConnectionError() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create handler that returns an error.
	handler := jsonrpc2.HandlerWithError(
		func(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) (interface{}, error) {
			return nil, &jsonrpc2.Error{
				Code:    -32001,
				Message: "Test error",
			}
		},
	)

	// Create connections.
	clientConn := jsonrpc2.NewConn(ctx, s.transport.ClientStream(), handler)
	agentConn := jsonrpc2.NewConn(ctx, s.transport.AgentStream(), handler)
	defer clientConn.Close()
	defer agentConn.Close()

	// Test error propagation.
	var result interface{}
	err := clientConn.Call(ctx, "errorMethod", nil, &result)
	s.Require().Error(err)

	// Check it's a JSON-RPC error.
	var jsonrpcErr *jsonrpc2.Error
	s.Require().ErrorAs(err, &jsonrpcErr)
	s.Equal(int64(-32001), jsonrpcErr.Code)
	s.Equal("Test error", jsonrpcErr.Message)
}

func (s *TransportTestSuite) TestNotificationHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create handler that records notifications.
	notifications := make([]string, 0)
	var mu sync.Mutex

	handler := jsonrpc2.HandlerWithError(
		func(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
			if req.Notif {
				mu.Lock()
				notifications = append(notifications, req.Method)
				mu.Unlock()
				return struct{}{}, nil
			}
			return "response", nil
		},
	)

	// Create connections.
	clientConn := jsonrpc2.NewConn(ctx, s.transport.ClientStream(), handler)
	agentConn := jsonrpc2.NewConn(ctx, s.transport.AgentStream(), handler)
	defer clientConn.Close()
	defer agentConn.Close()

	// Send notifications.
	err := clientConn.Notify(ctx, "notification1", map[string]string{"test": "data1"})
	s.Require().NoError(err)

	err = clientConn.Notify(ctx, "notification2", map[string]string{"test": "data2"})
	s.Require().NoError(err)

	// Give notifications time to be processed.
	time.Sleep(100 * time.Millisecond)

	// Verify notifications were received.
	mu.Lock()
	s.Len(notifications, 2)
	s.Contains(notifications, "notification1")
	s.Contains(notifications, "notification2")
	mu.Unlock()
}

func (s *TransportTestSuite) TestConnectionTimeout() {
	// Create a very short timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	handler := jsonrpc2.HandlerWithError(
		func(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) (interface{}, error) {
			// Simulate slow processing.
			time.Sleep(50 * time.Millisecond)
			return "response", nil
		},
	)

	// Create connections.
	clientConn := jsonrpc2.NewConn(context.Background(), s.transport.ClientStream(), handler)
	agentConn := jsonrpc2.NewConn(context.Background(), s.transport.AgentStream(), handler)
	defer clientConn.Close()
	defer agentConn.Close()

	// Test timeout.
	var result interface{}
	err := clientConn.Call(ctx, "slowMethod", nil, &result)
	s.Require().Error(err)
	s.Contains(err.Error(), "context deadline exceeded")
}

func (s *TransportTestSuite) TestMultipleConnections() {
	// Test that we can create and use multiple connection pairs.
	transport1 := NewMockTransport()
	transport2 := NewMockTransport()
	defer transport1.Close()
	defer transport2.Close()

	ctx := context.Background()
	handler := jsonrpc2.HandlerWithError(
		func(_ context.Context, _ *jsonrpc2.Conn, _ *jsonrpc2.Request) (interface{}, error) {
			return map[string]string{"connection": "response"}, nil
		},
	)

	// Create multiple connection pairs.
	client1 := jsonrpc2.NewConn(ctx, transport1.ClientStream(), handler)
	agent1 := jsonrpc2.NewConn(ctx, transport1.AgentStream(), handler)
	client2 := jsonrpc2.NewConn(ctx, transport2.ClientStream(), handler)
	agent2 := jsonrpc2.NewConn(ctx, transport2.AgentStream(), handler)

	defer func() {
		client1.Close()
		agent1.Close()
		client2.Close()
		agent2.Close()
	}()

	// Test that connections are independent.
	var result1, result2 map[string]interface{}

	err1 := client1.Call(ctx, "test1", map[string]string{"transport": "1"}, &result1)
	err2 := client2.Call(ctx, "test2", map[string]string{"transport": "2"}, &result2)

	s.Require().NoError(err1)
	s.Require().NoError(err2)
	s.Equal("response", result1["connection"])
	s.Equal("response", result2["connection"])
}

func TestTransportTestSuite(t *testing.T) {
	suite.Run(t, new(TransportTestSuite))
}
