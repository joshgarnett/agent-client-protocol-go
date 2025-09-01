package acp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HandlerRegistryTestSuite struct {
	suite.Suite

	registry *HandlerRegistry
}

func (s *HandlerRegistryTestSuite) SetupTest() {
	s.registry = NewHandlerRegistry()
}

func (s *HandlerRegistryTestSuite) TestRegisterMethod() {
	called := false

	// Register a test method.
	s.registry.RegisterMethod("test/method", func(_ context.Context, _ json.RawMessage) (any, error) {
		called = true
		return map[string]string{"result": "success"}, nil
	})

	// Verify registration doesn't panic and method is stored.
	s.False(called) // Should not be called during registration
	s.NotNil(s.registry.methods["test/method"])
}

func (s *HandlerRegistryTestSuite) TestRegisterNotification() {
	called := false

	// Register a test notification.
	s.registry.RegisterNotification("test/notification", func(_ context.Context, _ json.RawMessage) error {
		called = true
		return nil
	})

	// Verify registration.
	s.False(called) // Should not be called during registration
	s.NotNil(s.registry.notifications["test/notification"])
}

func (s *HandlerRegistryTestSuite) TestRegisterInitializeHandler() {
	called := false

	// Register initialize handler.
	s.registry.RegisterInitializeHandler(
		func(_ context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
			called = true
			return &api.InitializeResponse{
				ProtocolVersion: params.ProtocolVersion,
			}, nil
		},
	)

	// Verify registration.
	s.False(called)
	s.NotNil(s.registry.methods[api.MethodInitialize])
}

func (s *HandlerRegistryTestSuite) TestRegisterSessionNewHandler() {
	called := false

	s.registry.RegisterSessionNewHandler(
		func(_ context.Context, _ *api.NewSessionRequest) (*api.NewSessionResponse, error) {
			called = true
			return &api.NewSessionResponse{}, nil
		},
	)

	s.False(called)
	s.NotNil(s.registry.methods[api.MethodSessionNew])
}

func (s *HandlerRegistryTestSuite) TestRegisterFsReadTextFileHandler() {
	called := false

	s.registry.RegisterFsReadTextFileHandler(
		func(_ context.Context, _ *api.ReadTextFileRequest) (*api.ReadTextFileResponse, error) {
			called = true
			return &api.ReadTextFileResponse{}, nil
		},
	)

	s.False(called)
	s.NotNil(s.registry.methods[api.MethodFsReadTextFile])
}

func (s *HandlerRegistryTestSuite) TestRegisterFsWriteTextFileHandler() {
	called := false

	s.registry.RegisterFsWriteTextFileHandler(func(_ context.Context, _ *api.WriteTextFileRequest) error {
		called = true
		return nil
	})

	s.False(called)
	s.NotNil(s.registry.methods[api.MethodFsWriteTextFile])
}

func (s *HandlerRegistryTestSuite) TestHandlerCreation() {
	// Register some handlers.
	s.registry.RegisterMethod("test/method", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "result", nil
	})

	s.registry.RegisterNotification("test/notification", func(_ context.Context, _ json.RawMessage) error {
		return nil
	})

	// Create handler.
	handler := s.registry.Handler()
	s.NotNil(handler)
}

func TestHandlerRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerRegistryTestSuite))
}

// Integration tests using actual connections.

func TestHandlerRegistry_IntegrationInitialize(t *testing.T) {
	// Create connection pair.
	pair := NewConnectionPair(t)
	defer pair.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test initialize - CLIENT calls initialize on AGENT (using AgentConn which represents agent methods).
	request := SampleInitializeRequest()
	result, err := pair.AgentConn.Initialize(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, request.ProtocolVersion, result.ProtocolVersion)
	assert.True(t, result.AgentCapabilities.LoadSession)
}

func TestHandlerRegistry_IntegrationSessionNew(t *testing.T) {
	pair := NewConnectionPair(t)
	defer pair.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test session creation - CLIENT calls session/new on AGENT (using AgentConn which represents agent methods)
	request := SampleNewSessionRequest()
	result, err := pair.AgentConn.SessionNew(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.SessionId)

	// Verify session was recorded in test agent.
	sessions := pair.TestAgent.GetSessions()
	assert.Len(t, sessions, 1)
}

func TestHandlerRegistry_IntegrationFileOperations(t *testing.T) {
	pair := NewConnectionPair(t)
	defer pair.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add file content to test client.
	pair.TestClient.AddFileContent("/test/file.txt", "Hello, World!")

	// Test file read - AGENT calls fs/read_text_file on CLIENT (using ClientConn which represents client methods)
	readRequest := SampleReadTextFileRequest("session-1", "/test/file.txt")
	readResult, err := pair.ClientConn.FsReadTextFile(ctx, readRequest)

	require.NoError(t, err)
	require.NotNil(t, readResult)
	assert.Equal(t, "Hello, World!", readResult.Content)

	// Test file write - AGENT calls fs/write_text_file on CLIENT (using ClientConn which represents client methods)
	writeRequest := SampleWriteTextFileRequest("session-1", "/test/output.txt", "Written content")
	err = pair.ClientConn.FsWriteTextFile(ctx, writeRequest)

	require.NoError(t, err)

	// Verify write was recorded.
	writtenFiles := pair.TestClient.GetWrittenFiles()
	assert.Len(t, writtenFiles, 1)
	assert.Equal(t, "/test/output.txt", writtenFiles[0].Path)
	assert.Equal(t, "Written content", writtenFiles[0].Content)
}

func TestHandlerRegistry_IntegrationErrorHandling(t *testing.T) {
	pair := NewConnectionPair(t)
	defer pair.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Configure test client to return error.
	pair.TestClient.SetShouldError("fs/read_text_file", true)

	// Test error propagation - AGENT calls fs/read_text_file on CLIENT (using ClientConn which represents client methods)
	readRequest := SampleReadTextFileRequest("session-1", "/test/file.txt")
	result, err := pair.ClientConn.FsReadTextFile(ctx, readRequest)

	require.Error(t, err)
	assert.Nil(t, result)

	// Check it's the expected ACP error.
	AssertACPError(t, err, api.ErrorCodeNotFound)
}
