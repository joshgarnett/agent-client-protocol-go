package acp

import (
	"context"
	"testing"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"

	"github.com/stretchr/testify/suite"
)

// ProtocolFlowTestSuite tests complete ACP protocol flows.
type ProtocolFlowTestSuite struct {
	suite.Suite

	pair *ConnectionPair
}

func (s *ProtocolFlowTestSuite) SetupTest() {
	s.pair = NewConnectionPair(s.T())
}

func (s *ProtocolFlowTestSuite) TearDownTest() {
	if s.pair != nil {
		s.pair.Close()
	}
}

func (s *ProtocolFlowTestSuite) TestCompleteInitializationFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Initialize - client calls initialize on agent.
	request := SampleInitializeRequest()
	response, err := s.pair.AgentConn.Initialize(ctx, request)

	s.Require().NoError(err)
	s.Require().NotNil(response)
	s.Equal(request.ProtocolVersion, response.ProtocolVersion)
	s.True(response.AgentCapabilities.LoadSession)
	s.NotEmpty(response.AuthMethods)

	// 2. Authenticate - client authenticates with agent.
	s.Require().NotEmpty(response.AuthMethods)
	authRequest := &api.AuthenticateRequest{
		MethodId: response.AuthMethods[0].Id,
	}
	err = s.pair.AgentConn.Authenticate(ctx, authRequest)
	s.Require().NoError(err)

	// Verify agent is authenticated.
	s.True(s.pair.TestAgent.IsAuthenticated())
}

func (s *ProtocolFlowTestSuite) TestCompleteSessionFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize first.
	s.initializeConnection(ctx)

	// 1. Create new session.
	sessionRequest := SampleNewSessionRequest()
	sessionResponse, err := s.pair.AgentConn.SessionNew(ctx, sessionRequest)

	s.Require().NoError(err)
	s.Require().NotNil(sessionResponse)
	s.NotEmpty(sessionResponse.SessionId)

	// Verify session was created.
	sessions := s.pair.TestAgent.GetSessions()
	s.Len(sessions, 1)

	sessionID := string(sessionResponse.SessionId)
	session, exists := sessions[sessionID]
	s.True(exists)
	s.Equal(sessionRequest.Cwd, session.CWD)

	// 2. Load existing session.
	loadRequest := &api.LoadSessionRequest{
		SessionId: sessionResponse.SessionId,
	}
	err = s.pair.AgentConn.SessionLoad(ctx, loadRequest)
	s.Require().NoError(err)
}

func (s *ProtocolFlowTestSuite) TestCompleteFileOperationFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize connection.
	s.initializeConnection(ctx)

	// Set up test files in client.
	s.pair.TestClient.AddFileContent("/project/src/main.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`)
	s.pair.TestClient.AddFileContent("/project/README.md", "# My Project")

	// 1. Agent reads multiple files.
	files := []string{"/project/src/main.go", "/project/README.md", "/project/nonexistent.txt"}

	for _, filePath := range files {
		readRequest := SampleReadTextFileRequest("session-1", filePath)
		result, err := s.pair.ClientConn.FsReadTextFile(ctx, readRequest)

		if filePath == "/project/nonexistent.txt" {
			// This should return default content since file doesn't exist.
			s.Require().NoError(err)
			s.Equal("default content", result.Content)
		} else {
			s.Require().NoError(err)
			s.NotEmpty(result.Content)
		}
	}

	// 2. Agent writes files.
	writeFiles := map[string]string{
		"/project/output.txt":     "Generated output",
		"/project/config.json":    `{"debug": true}`,
		"/project/logs/debug.log": "Debug information",
	}

	for filePath, content := range writeFiles {
		writeRequest := SampleWriteTextFileRequest("session-1", filePath, content)
		err := s.pair.ClientConn.FsWriteTextFile(ctx, writeRequest)
		s.Require().NoError(err)
	}

	// Verify all files were written.
	writtenFiles := s.pair.TestClient.GetWrittenFiles()
	s.Len(writtenFiles, len(writeFiles))

	for _, written := range writtenFiles {
		expectedContent, exists := writeFiles[written.Path]
		s.True(exists, "Unexpected file written: %s", written.Path)
		s.Equal(expectedContent, written.Content)
	}
}

func (s *ProtocolFlowTestSuite) TestPromptFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize and create session.
	s.initializeConnection(ctx)
	sessionResponse := s.createSession(ctx)

	// Send prompt request.
	promptRequest := SamplePromptRequest(string(sessionResponse.SessionId))
	promptResponse, err := s.pair.AgentConn.SessionPrompt(ctx, promptRequest)

	s.Require().NoError(err)
	s.NotNil(promptResponse)

	// Verify prompt was recorded.
	prompts := s.pair.TestAgent.GetPromptsReceived()
	s.Len(prompts, 1)
	s.Equal(string(sessionResponse.SessionId), prompts[0].SessionID)
}

func (s *ProtocolFlowTestSuite) TestErrorRecoveryFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize connection.
	s.initializeConnection(ctx)

	// 1. Test failed file operation recovery.
	s.pair.TestClient.SetShouldError("fs/read_text_file", true)

	readRequest := SampleReadTextFileRequest("session-1", "/test/file.txt")
	result, err := s.pair.ClientConn.FsReadTextFile(ctx, readRequest)

	s.Require().Error(err)
	s.Nil(result)
	AssertACPError(s.T(), err, api.ErrorCodeNotFound)

	// 2. Recover from error - disable error simulation.
	s.pair.TestClient.SetShouldError("fs/read_text_file", false)
	s.pair.TestClient.AddFileContent("/test/file.txt", "Recovered content")

	result, err = s.pair.ClientConn.FsReadTextFile(ctx, readRequest)
	s.Require().NoError(err)
	s.NotNil(result)
	s.Equal("Recovered content", result.Content)

	// 3. Test agent error recovery.
	s.pair.TestAgent.SetShouldError("session/new", true)

	sessionRequest := SampleNewSessionRequest()
	sessionResponse, err := s.pair.AgentConn.SessionNew(ctx, sessionRequest)

	s.Require().Error(err)
	s.Nil(sessionResponse)

	// Recover agent.
	s.pair.TestAgent.SetShouldError("session/new", false)

	sessionResponse, err = s.pair.AgentConn.SessionNew(ctx, sessionRequest)
	s.Require().NoError(err)
	s.NotNil(sessionResponse)
}

func (s *ProtocolFlowTestSuite) TestCancellationFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize and create session.
	s.initializeConnection(ctx)
	sessionResponse := s.createSession(ctx)

	// Send cancellation notification.
	cancelRequest := &api.CancelNotification{
		SessionId: sessionResponse.SessionId,
	}
	err := s.pair.AgentConn.SessionCancel(ctx, cancelRequest)
	s.Require().NoError(err)

	// Give notification time to be processed.
	time.Sleep(100 * time.Millisecond)

	// Verify cancellation was received.
	cancellations := s.pair.TestAgent.GetCancellationsReceived()
	if len(cancellations) == 0 {
		s.T().Log("No cancellations received, notification might not be working")
		s.T().FailNow()
	}
	s.Len(cancellations, 1)
	s.Equal(string(sessionResponse.SessionId), cancellations[0])
}

// Helper methods.

func (s *ProtocolFlowTestSuite) initializeConnection(ctx context.Context) *api.InitializeResponse {
	request := SampleInitializeRequest()
	response, err := s.pair.AgentConn.Initialize(ctx, request)
	s.Require().NoError(err)
	s.Require().NotNil(response)
	return response
}

func (s *ProtocolFlowTestSuite) createSession(ctx context.Context) *api.NewSessionResponse {
	request := SampleNewSessionRequest()
	response, err := s.pair.AgentConn.SessionNew(ctx, request)
	s.Require().NoError(err)
	s.Require().NotNil(response)
	return response
}

func TestProtocolFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ProtocolFlowTestSuite))
}
