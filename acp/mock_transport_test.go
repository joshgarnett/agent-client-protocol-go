package acp

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
)

// MockTransport provides an in-memory bidirectional transport for testing.
type MockTransport struct {
	clientToAgent *pipe
	agentToClient *pipe
}

// NewMockTransport creates a new mock transport with bidirectional pipes.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		clientToAgent: newPipe(),
		agentToClient: newPipe(),
	}
}

// ClientStream returns the ObjectStream for the client side.
func (m *MockTransport) ClientStream() jsonrpc2.ObjectStream {
	return &mockObjectStream{
		reader: m.agentToClient,
		writer: m.clientToAgent,
	}
}

// AgentStream returns the ObjectStream for the agent side.
func (m *MockTransport) AgentStream() jsonrpc2.ObjectStream {
	return &mockObjectStream{
		reader: m.clientToAgent,
		writer: m.agentToClient,
	}
}

// Close closes both pipes.
func (m *MockTransport) Close() error {
	m.clientToAgent.Close()
	m.agentToClient.Close()
	return nil
}

// pipe is an in-memory pipe for passing objects between sides.
type pipe struct {
	ch     chan json.RawMessage
	closed bool
	mu     sync.RWMutex
}

func newPipe() *pipe {
	return &pipe{
		ch: make(chan json.RawMessage, 100), // Buffered to avoid blocking
	}
}

func (p *pipe) Write(obj json.RawMessage) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return io.ErrClosedPipe
	}

	select {
	case p.ch <- obj:
		return nil
	default:
		return io.ErrShortBuffer
	}
}

func (p *pipe) Read() (json.RawMessage, error) {
	obj, ok := <-p.ch
	if !ok {
		return nil, io.EOF
	}
	return obj, nil
}

func (p *pipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed {
		p.closed = true
		close(p.ch)
	}
	return nil
}

// mockObjectStream implements jsonrpc2.ObjectStream using pipes.
type mockObjectStream struct {
	reader *pipe
	writer *pipe
}

func (s *mockObjectStream) WriteObject(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return s.writer.Write(json.RawMessage(data))
}

func (s *mockObjectStream) ReadObject(obj interface{}) error {
	data, err := s.reader.Read()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func (s *mockObjectStream) Close() error {
	// Don't close the pipes directly as they might be shared.
	return nil
}
