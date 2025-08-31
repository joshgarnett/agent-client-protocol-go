package acp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/require"
)

// ConnectionPair holds both sides of a test connection.
type ConnectionPair struct {
	AgentConn     *AgentConnection
	ClientConn    *ClientConnection
	Transport     *MockTransport
	AgentHandler  *HandlerRegistry
	ClientHandler *HandlerRegistry
	TestAgent     *TestAgent
	TestClient    *TestClient
}

// NewConnectionPair creates a connected agent-client pair for testing.
func NewConnectionPair(t *testing.T) *ConnectionPair {
	t.Helper()

	// Create mock transport.
	transport := NewMockTransport()

	// Create test implementations.
	testAgent := NewTestAgent()
	testClient := NewTestClient()

	// Create handler registries.
	agentHandler := NewHandlerRegistry()
	clientHandler := NewHandlerRegistry()

	// Register agent handlers (these handle requests FROM client TO agent).
	agentHandler.RegisterInitializeHandler(testAgent.HandleInitialize)
	agentHandler.RegisterAuthenticateHandler(testAgent.HandleAuthenticate)
	agentHandler.RegisterSessionNewHandler(testAgent.HandleSessionNew)
	agentHandler.RegisterSessionLoadHandler(testAgent.HandleSessionLoad)
	agentHandler.RegisterSessionPromptHandler(testAgent.HandleSessionPrompt)
	agentHandler.RegisterNotification(MethodSessionCancel, func(ctx context.Context, params json.RawMessage) error {
		var cancelParams CancelNotification
		if err := json.Unmarshal(params, &cancelParams); err != nil {
			return err
		}
		return testAgent.HandleSessionCancel(ctx, &cancelParams)
	})

	// Register client handlers (these handle requests FROM agent TO client).
	clientHandler.RegisterFsReadTextFileHandler(testClient.HandleFsReadTextFile)
	clientHandler.RegisterFsWriteTextFileHandler(testClient.HandleFsWriteTextFile)

	// Create connections:.
	// 1. Agent side: receives agent requests (initialize, session/new) and sends client requests (fs/read, fs/write)
	// 2. Client side: receives client requests (fs/read, fs/write) and sends agent requests (initialize, session/new)
	ctx := context.Background()

	// The "agent side" connection - receives calls to agent methods, can send calls to client methods.
	agentSideConn := NewClientConnection(ctx, transport.AgentStream(), agentHandler.Handler())

	// The "client side" connection - receives calls to client methods, can send calls to agent methods.
	clientSideConn := NewAgentConnection(ctx, transport.ClientStream(), clientHandler.Handler())

	return &ConnectionPair{
		AgentConn:     clientSideConn, // Connection that can call agent methods
		ClientConn:    agentSideConn,  // Connection that can call client methods
		Transport:     transport,
		AgentHandler:  agentHandler,
		ClientHandler: clientHandler,
		TestAgent:     testAgent,
		TestClient:    testClient,
	}
}

// Close closes the connection pair.
func (cp *ConnectionPair) Close() error {
	cp.AgentConn.Close()
	cp.ClientConn.Close()
	return cp.Transport.Close()
}

// WaitWithTimeout waits for a condition with a timeout.
func WaitWithTimeout(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		<-ticker.C
		if time.Now().After(deadline) {
			t.Fatalf("Timeout waiting for condition: %s", message)
			return
		}
	}
}

// Common test data.

// SampleInitializeRequest creates a sample initialize request.
func SampleInitializeRequest() *InitializeRequest {
	return &InitializeRequest{
		ProtocolVersion: ProtocolVersion(ACPProtocolVersion),
		ClientCapabilities: ClientCapabilities{
			Fs: FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
		},
	}
}

// SampleNewSessionRequest creates a sample new session request.
func SampleNewSessionRequest() *NewSessionRequest {
	return &NewSessionRequest{
		Cwd: "/test/project",
		// McpServers field might not exist in current schema.
	}
}

// SamplePromptRequest creates a sample prompt request.
func SamplePromptRequest(sessionID string) *PromptRequest {
	return &PromptRequest{
		SessionId: SessionId(sessionID),
		Prompt:    []PromptRequestPromptElem{
			// Add sample prompt elements based on generated types.
		},
	}
}

// SampleReadTextFileRequest creates a sample read file request.
func SampleReadTextFileRequest(sessionID, path string) *ReadTextFileRequest {
	return &ReadTextFileRequest{
		SessionId: SessionId(sessionID),
		Path:      path,
		Line:      nil,
		Limit:     nil,
	}
}

// SampleWriteTextFileRequest creates a sample write file request.
func SampleWriteTextFileRequest(sessionID, path, content string) *WriteTextFileRequest {
	return &WriteTextFileRequest{
		SessionId: SessionId(sessionID),
		Path:      path,
		Content:   content,
	}
}

// AssertNoError is a helper that fails the test if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	require.NoError(t, err)
}

// AssertError is a helper that fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
}

// AssertACPError is a helper that asserts an error is an ACP error with specific code.
func AssertACPError(t *testing.T, err error, expectedCode int) {
	t.Helper()
	require.Error(t, err)

	// Check if it's a direct ACPError.
	var acpErr *ACPError
	if errors.As(err, &acpErr) {
		require.Equal(t, expectedCode, acpErr.Code)
		return
	}

	// Check if it's a jsonrpc2.Error containing an ACP error code.
	var jsonrpcErr *jsonrpc2.Error
	if errors.As(err, &jsonrpcErr) {
		require.Equal(t, int64(expectedCode), jsonrpcErr.Code)
		return
	}

	t.Fatalf("Expected ACP error or jsonrpc2.Error, got %T: %v", err, err)
}
