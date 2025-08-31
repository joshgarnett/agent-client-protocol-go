package acp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ErrorHandlingTestSuite provides comprehensive error code testing.
type ErrorHandlingTestSuite struct {
	suite.Suite

	pair *ConnectionPair
}

func (s *ErrorHandlingTestSuite) SetupTest() {
	s.pair = NewConnectionPair(s.T())
}

func (s *ErrorHandlingTestSuite) TearDownTest() {
	if s.pair != nil {
		s.pair.Close()
	}
}

func (s *ErrorHandlingTestSuite) TestInitializationErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.pair.TestAgent.SetShouldError("initialize", true)

	request := SampleInitializeRequest()
	result, err := s.pair.AgentConn.Initialize(ctx, request)

	s.Require().Error(err)
	s.Nil(result)
	AssertACPError(s.T(), err, ErrorCodeInitializationError)
}

func (s *ErrorHandlingTestSuite) TestAuthenticationErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestAgent.SetShouldError("authenticate", true)

	authRequest := &AuthenticateRequest{
		MethodId: AuthMethodId("invalid-method"),
	}
	err := s.pair.AgentConn.Authenticate(ctx, authRequest)

	s.Require().Error(err)
	AssertACPError(s.T(), err, ErrorCodeUnauthorized)
}

func (s *ErrorHandlingTestSuite) TestSessionErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestAgent.SetShouldError("session/new", true)

	request := SampleNewSessionRequest()
	result, err := s.pair.AgentConn.SessionNew(ctx, request)

	s.Require().Error(err)
	s.Nil(result)
	AssertACPError(s.T(), err, ErrorCodeInternalServerError)

	s.pair.TestAgent.SetShouldError("session/new", false)
	sessionResponse, err := s.pair.AgentConn.SessionNew(ctx, request)
	s.Require().NoError(err)

	s.pair.TestAgent.SetShouldError("session/load", true)

	loadRequest := &LoadSessionRequest{
		SessionId: SessionId("nonexistent-session"),
	}
	err = s.pair.AgentConn.SessionLoad(ctx, loadRequest)

	s.Require().Error(err)
	AssertACPError(s.T(), err, ErrorCodeNotFound)

	s.pair.TestAgent.SetShouldError("session/load", false)
	loadRequest.SessionId = sessionResponse.SessionId
	err = s.pair.AgentConn.SessionLoad(ctx, loadRequest)
	s.Require().NoError(err)
}

func (s *ErrorHandlingTestSuite) TestFileSystemErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestClient.SetShouldError("fs/read_text_file", true)

	readRequest := SampleReadTextFileRequest("session-1", "/nonexistent/file.txt")
	result, err := s.pair.ClientConn.FsReadTextFile(ctx, readRequest)

	s.Require().Error(err)
	s.Nil(result)
	AssertACPError(s.T(), err, ErrorCodeNotFound)

	s.pair.TestClient.SetShouldError("fs/write_text_file", true)

	writeRequest := SampleWriteTextFileRequest("session-1", "/forbidden/file.txt", "content")
	err = s.pair.ClientConn.FsWriteTextFile(ctx, writeRequest)

	s.Require().Error(err)
	AssertACPError(s.T(), err, ErrorCodeForbidden)
}

func (s *ErrorHandlingTestSuite) TestPromptErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestAgent.SetShouldError("session/prompt", true)

	promptRequest := SamplePromptRequest("session-1")
	result, err := s.pair.AgentConn.SessionPrompt(ctx, promptRequest)

	s.Require().Error(err)
	s.Nil(result)
	AssertACPError(s.T(), err, ErrorCodeInternalServerError)
}

// Permission operations are not yet implemented as RPC methods.
// (they exist in schema but aren't registered in the test setup).

// Terminal operations are not yet implemented as RPC methods.
// (they exist in schema but don't have x-method fields).

func (s *ErrorHandlingTestSuite) TestAllErrorCodes() {
	errorCodes := []struct {
		code        int
		description string
	}{
		{ErrorCodeInitializationError, "Initialization Error"},
		{ErrorCodeUnauthorized, "Unauthorized"},
		{ErrorCodeForbidden, "Forbidden"},
		{ErrorCodeNotFound, "Not Found"},
		{ErrorCodeConflict, "Conflict"},
		{ErrorCodeTooManyRequests, "Too Many Requests"},
		{ErrorCodeInternalServerError, "Internal Server Error"},
	}

	for _, tc := range errorCodes {
		s.Run(tc.description, func() {
			err := &ACPError{
				Code:    tc.code,
				Message: tc.description,
			}

			s.Equal(tc.code, err.Code)
			s.Contains(err.Error(), tc.description)
		})
	}
}

func (s *ErrorHandlingTestSuite) TestErrorRecovery() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestClient.SetShouldError("fs/read_text_file", true)

	readRequest := SampleReadTextFileRequest("session-1", "/test/file.txt")
	_, err := s.pair.ClientConn.FsReadTextFile(ctx, readRequest)
	s.Require().Error(err)
	AssertACPError(s.T(), err, ErrorCodeNotFound)

	s.pair.TestClient.SetShouldError("fs/read_text_file", false)
	s.pair.TestClient.AddFileContent("/test/file.txt", "recovered content")

	result, err := s.pair.ClientConn.FsReadTextFile(ctx, readRequest)
	s.Require().NoError(err)
	s.NotNil(result)
	s.Equal("recovered content", result.Content)
}

func (s *ErrorHandlingTestSuite) TestMixedSuccessAndFailure() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.initializeConnection(ctx)

	s.pair.TestClient.AddFileContent("/success/file1.txt", "content1")
	s.pair.TestClient.AddFileContent("/success/file2.txt", "content2")

	testCases := []struct {
		path          string
		shouldSucceed bool
		expectedError int
	}{
		{"/success/file1.txt", true, 0},
		{"/success/file2.txt", true, 0},
		{"/fail/file1.txt", false, ErrorCodeNotFound},
		{"/fail/file2.txt", false, ErrorCodeNotFound},
	}

	s.pair.TestClient.SetShouldError("fs/read_text_file", false)

	for i, tc := range testCases {
		s.Run(tc.path, func() {
			if !tc.shouldSucceed {
				s.pair.TestClient.SetShouldError("fs/read_text_file", true)
			} else {
				s.pair.TestClient.SetShouldError("fs/read_text_file", false)
			}

			request := SampleReadTextFileRequest("session-1", tc.path)
			result, err := s.pair.ClientConn.FsReadTextFile(ctx, request)

			if tc.shouldSucceed {
				s.Require().NoError(err)
				s.NotNil(result)
			} else {
				s.Require().Error(err)
				s.Nil(result)
				AssertACPError(s.T(), err, tc.expectedError)
			}
		})

		if i < len(testCases)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *ErrorHandlingTestSuite) initializeConnection(ctx context.Context) *InitializeResponse {
	request := SampleInitializeRequest()
	response, err := s.pair.AgentConn.Initialize(ctx, request)
	s.Require().NoError(err)
	s.Require().NotNil(response)
	return response
}

func TestErrorHandlingTestSuite(t *testing.T) {
	suite.Run(t, new(ErrorHandlingTestSuite))
}
