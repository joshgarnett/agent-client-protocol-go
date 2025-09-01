package acp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
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
	agentHandler.RegisterSessionCancelHandler(testAgent.HandleSessionCancel)

	// Register client handlers (these handle requests FROM agent TO client).
	clientHandler.RegisterFsReadTextFileHandler(testClient.HandleFsReadTextFile)
	clientHandler.RegisterFsWriteTextFileHandler(testClient.HandleFsWriteTextFile)

	ctx := context.Background()

	// The "agent side" connection - receives calls to agent methods, can send calls to client methods.
	agentSideConn, err := NewClientConnectionStdio(ctx, transport.Agent(), agentHandler, testRequestTimeout)
	if err != nil {
		t.Fatalf("Failed to create agent side connection: %v", err)
	}

	// The "client side" connection - receives calls to client methods, can send calls to agent methods.
	clientSideConn, err := NewAgentConnectionStdio(ctx, transport.Client(), clientHandler, testRequestTimeout)
	if err != nil {
		t.Fatalf("Failed to create client side connection: %v", err)
	}

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
func SampleInitializeRequest() *api.InitializeRequest {
	return &api.InitializeRequest{
		ProtocolVersion: api.ProtocolVersion(api.ACPProtocolVersion),
		ClientCapabilities: api.ClientCapabilities{
			Fs: api.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
		},
	}
}

// SampleNewSessionRequest creates a sample new session request.
func SampleNewSessionRequest() *api.NewSessionRequest {
	return &api.NewSessionRequest{
		Cwd: "/test/project",
		McpServers: []api.McpServer{
			{
				Name:    "filesystem",
				Command: "/usr/local/bin/mcp-server-filesystem",
				Args:    []string{"--stdio"},
				Env:     []api.EnvVariable{},
			},
		},
	}
}

// SamplePromptRequest creates a sample prompt request.
func SamplePromptRequest(sessionID string) *api.PromptRequest {
	return &api.PromptRequest{
		SessionId: api.SessionId(sessionID),
		Prompt:    []api.PromptRequestPromptElem{
			// Add sample prompt elements based on generated types.
		},
	}
}

// SampleReadTextFileRequest creates a sample read file request.
func SampleReadTextFileRequest(sessionID, path string) *api.ReadTextFileRequest {
	return &api.ReadTextFileRequest{
		SessionId: api.SessionId(sessionID),
		Path:      path,
		Line:      nil,
		Limit:     nil,
	}
}

// SampleWriteTextFileRequest creates a sample write file request.
func SampleWriteTextFileRequest(sessionID, path, content string) *api.WriteTextFileRequest {
	return &api.WriteTextFileRequest{
		SessionId: api.SessionId(sessionID),
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
	var acpErr *api.ACPError
	if errors.As(err, &acpErr) {
		require.Equal(t, expectedCode, acpErr.Code)
		return
	}

	// Check error message contains expected ACP error code format: "ACP Error -32004: ..."
	errMsg := err.Error()
	expectedCodeStr := fmt.Sprintf("ACP Error %d:", expectedCode)
	require.Contains(
		t,
		errMsg,
		expectedCodeStr,
		"Expected error to contain ACP error code %d, got: %v",
		expectedCode,
		err,
	)
}
