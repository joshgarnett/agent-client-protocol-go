package acp

import (
	"io"
)

// MockTransport provides an in-memory bidirectional transport for testing.
type MockTransport struct {
	clientRWC io.ReadWriteCloser
	agentRWC  io.ReadWriteCloser
}

// NewMockTransport creates a new mock transport using in-memory pipes.
func NewMockTransport() *MockTransport {
	// Pipe for client writing to agent
	agentRead, clientWrite := io.Pipe()
	// Pipe for agent writing to client
	clientRead, agentWrite := io.Pipe()

	return &MockTransport{
		clientRWC: &pipeReadWriteCloser{
			reader: clientRead,
			writer: clientWrite,
		},
		agentRWC: &pipeReadWriteCloser{
			reader: agentRead,
			writer: agentWrite,
		},
	}
}

// Client returns an io.ReadWriteCloser for the client side of the transport.
func (m *MockTransport) Client() io.ReadWriteCloser {
	return m.clientRWC
}

// Agent returns an io.ReadWriteCloser for the agent side of the transport.
func (m *MockTransport) Agent() io.ReadWriteCloser {
	return m.agentRWC
}

// Close closes both pipes.
func (m *MockTransport) Close() error {
	// Closing the ReadWriteClosers will close the underlying pipes.
	m.clientRWC.Close()
	m.agentRWC.Close()
	return nil
}

// pipeReadWriteCloser combines a reader and a writer to form a bidirectional stream.
type pipeReadWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (prwc *pipeReadWriteCloser) Read(p []byte) (int, error) {
	return prwc.reader.Read(p)
}

func (prwc *pipeReadWriteCloser) Write(p []byte) (int, error) {
	return prwc.writer.Write(p)
}

func (prwc *pipeReadWriteCloser) Close() error {
	// Closing both ends of the pipe is often necessary to unblock any readers/writers.
	werr := prwc.writer.Close()
	rerr := prwc.reader.Close()
	if werr != nil {
		return werr
	}
	return rerr
}
