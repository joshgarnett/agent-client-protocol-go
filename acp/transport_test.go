package acp

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"

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
	// Get both sides of the transport.
	clientRWC := s.transport.Client()
	agentRWC := s.transport.Agent()

	// Test client to agent communication.
	testObj := map[string]interface{}{
		"method": "test",
		"params": []string{"hello", "world"},
	}
	data, err := json.Marshal(testObj)
	s.Require().NoError(err)

	// Client writes, agent should read (async to avoid pipe deadlock).
	writeComplete := make(chan error, 1)
	go func() {
		_, writeErr := clientRWC.Write(data)
		writeComplete <- writeErr
	}()

	buffer := make([]byte, 1024)
	n, err := agentRWC.Read(buffer)
	s.Require().NoError(err)

	// Wait for write to complete
	err = <-writeComplete
	s.Require().NoError(err)

	var received map[string]interface{}
	err = json.Unmarshal(buffer[:n], &received)
	s.Require().NoError(err)
	s.Equal("test", received["method"])
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
	data, err = json.Marshal(responseObj)
	s.Require().NoError(err)

	// Agent writes, client should read (async to avoid pipe deadlock).
	go func() {
		_, writeErr := agentRWC.Write(data)
		writeComplete <- writeErr
	}()

	n, err = clientRWC.Read(buffer)
	s.Require().NoError(err)

	// Wait for write to complete
	err = <-writeComplete
	s.Require().NoError(err)

	var responseReceived map[string]interface{}
	err = json.Unmarshal(buffer[:n], &responseReceived)
	s.Require().NoError(err)
	s.Equal("success", responseReceived["result"])
	s.InDelta(float64(123), responseReceived["id"], 0.01)
}

func (s *TransportTestSuite) TestMockTransportClosure() {
	clientRWC := s.transport.Client()
	agentRWC := s.transport.Agent()

	// Close transport.
	err := s.transport.Close()
	s.Require().NoError(err)

	// Reads should return an error after close.
	buffer := make([]byte, 1024)
	_, err = clientRWC.Read(buffer)
	s.Require().Error(err) // Could be EOF or "io: read/write on closed pipe"

	_, err = agentRWC.Read(buffer)
	s.Require().Error(err) // Could be EOF or "io: read/write on closed pipe"

	// Writes should fail after close.
	_, err = clientRWC.Write([]byte("test"))
	s.Equal(io.ErrClosedPipe, err)

	_, err = agentRWC.Write([]byte("test"))
	s.Equal(io.ErrClosedPipe, err)
}

func (s *TransportTestSuite) TestMockTransportConcurrentAccess() {
	clientRWC := s.transport.Client()
	agentRWC := s.transport.Agent()

	const numMessages = 100
	var wg sync.WaitGroup

	// Channel to collect received messages.
	received := make(chan []byte, numMessages)

	// Start reader goroutine (agent reading from client).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range numMessages {
			buffer := make([]byte, 1024)
			n, err := agentRWC.Read(buffer)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				s.T().Errorf("Read error: %v", err)
				return
			}
			received <- buffer[:n]
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
				}
				data, err := json.Marshal(obj)
				if err != nil {
					s.T().Errorf("Marshal error: %v", err)
					return
				}
				_, err = clientRWC.Write(data)
				if err != nil {
					s.T().Errorf("Write error: %v", err)
					return
				}
			}
		}(w)
	}

	// Wait for all writers to complete.
	wg.Wait()

	// Verify all messages were received.
	close(received)
	receivedCount := 0
	for range received {
		receivedCount++
	}
	s.Equal(numMessages, receivedCount)
}

func TestTransportTestSuite(t *testing.T) {
	suite.Run(t, new(TransportTestSuite))
}
