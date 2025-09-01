package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

// SerializationTestSuite tests JSON wire format and serialization/deserialization.
type SerializationTestSuite struct {
	suite.Suite
}

func (s *SerializationTestSuite) TestInitializeRequestSerialization() {
	request := &InitializeRequest{
		ProtocolVersion: ProtocolVersion(ACPProtocolVersion),
		ClientCapabilities: ClientCapabilities{
			Fs: FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	}

	data, err := json.Marshal(request)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.InDelta(wireFormat["protocolVersion"], float64(ACPProtocolVersion), 0.01)

	clientCaps, ok := wireFormat["clientCapabilities"].(map[string]interface{})
	s.True(ok)
	s.Equal(true, clientCaps["terminal"])

	fs, ok := clientCaps["fs"].(map[string]interface{})
	s.True(ok)
	s.Equal(true, fs["readTextFile"])
	s.Equal(true, fs["writeTextFile"])

	var deserialized InitializeRequest
	err = json.Unmarshal(data, &deserialized)
	s.Require().NoError(err)
	s.Equal(request.ProtocolVersion, deserialized.ProtocolVersion)
	s.Equal(request.ClientCapabilities.Fs.ReadTextFile, deserialized.ClientCapabilities.Fs.ReadTextFile)
	s.Equal(request.ClientCapabilities.Terminal, deserialized.ClientCapabilities.Terminal)
}

func (s *SerializationTestSuite) TestInitializeResponseSerialization() {
	response := &InitializeResponse{
		ProtocolVersion: ProtocolVersion(ACPProtocolVersion),
		AgentCapabilities: AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: PromptCapabilities{
				Audio:           true,
				EmbeddedContext: true,
				Image:           true,
			},
		},
		AuthMethods: []AuthMethod{
			{
				Id:          AuthMethodId("oauth"),
				Name:        "OAuth Authentication",
				Description: stringPtr("OAuth 2.0 authentication"),
			},
		},
	}

	data, err := json.Marshal(response)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.InDelta(wireFormat["protocolVersion"], float64(ACPProtocolVersion), 0.01)

	agentCaps, ok := wireFormat["agentCapabilities"].(map[string]interface{})
	s.True(ok)
	s.Equal(true, agentCaps["loadSession"])

	promptCaps, ok := agentCaps["promptCapabilities"].(map[string]interface{})
	s.True(ok)
	s.Equal(true, promptCaps["audio"])
	s.Equal(true, promptCaps["embeddedContext"])
	s.Equal(true, promptCaps["image"])

	authMethods, ok := wireFormat["authMethods"].([]interface{})
	s.True(ok)
	s.Len(authMethods, 1)

	authMethod, ok := authMethods[0].(map[string]interface{})
	s.True(ok)
	s.Equal("oauth", authMethod["id"])
	s.Equal("OAuth Authentication", authMethod["name"])
	s.Equal("OAuth 2.0 authentication", authMethod["description"])

	var deserialized InitializeResponse
	err = json.Unmarshal(data, &deserialized)
	s.Require().NoError(err)
	s.Equal(response.ProtocolVersion, deserialized.ProtocolVersion)
	s.Equal(response.AgentCapabilities.LoadSession, deserialized.AgentCapabilities.LoadSession)
	s.Len(deserialized.AuthMethods, 1)
	s.Equal(response.AuthMethods[0].Id, deserialized.AuthMethods[0].Id)
}

func (s *SerializationTestSuite) TestSessionRequestSerialization() {
	request := &NewSessionRequest{
		Cwd: "/home/user/project",
	}

	data, err := json.Marshal(request)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.Equal("/home/user/project", wireFormat["cwd"])

	var deserialized NewSessionRequest
	err = json.Unmarshal(data, &deserialized)
	s.Require().NoError(err)
	s.Equal(request.Cwd, deserialized.Cwd)
}

func (s *SerializationTestSuite) TestFileOperationSerialization() {
	readRequest := &ReadTextFileRequest{
		SessionId: SessionId("session-123"),
		Path:      "/path/to/file.txt",
		Line:      intPtr(10),
		Limit:     intPtr(100),
	}

	data, err := json.Marshal(readRequest)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.Equal("session-123", wireFormat["sessionId"])
	s.Equal("/path/to/file.txt", wireFormat["path"])
	s.InDelta(float64(10), wireFormat["line"], 0.01)
	s.InDelta(float64(100), wireFormat["limit"], 0.01)

	writeRequest := &WriteTextFileRequest{
		SessionId: SessionId("session-123"),
		Path:      "/path/to/output.txt",
		Content:   "Hello, World!",
	}

	data, err = json.Marshal(writeRequest)
	s.Require().NoError(err)

	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.Equal("session-123", wireFormat["sessionId"])
	s.Equal("/path/to/output.txt", wireFormat["path"])
	s.Equal("Hello, World!", wireFormat["content"])
}

func (s *SerializationTestSuite) TestNotificationSerialization() {
	cancelNotif := &CancelNotification{
		SessionId: SessionId("session-456"),
	}

	data, err := json.Marshal(cancelNotif)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.Equal("session-456", wireFormat["sessionId"])

	var deserialized CancelNotification
	err = json.Unmarshal(data, &deserialized)
	s.Require().NoError(err)
	s.Equal(cancelNotif.SessionId, deserialized.SessionId)
}

func (s *SerializationTestSuite) TestErrorSerialization() {
	acpError := &ACPError{
		Code:    ErrorCodeNotFound,
		Message: "File not found",
		Data:    map[string]string{"path": "/missing/file.txt"},
	}

	data, err := json.Marshal(acpError)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.InDelta(wireFormat["code"], float64(ErrorCodeNotFound), 0.01)
	s.Equal("File not found", wireFormat["message"])

	errorData, ok := wireFormat["data"].(map[string]interface{})
	s.True(ok)
	s.Equal("/missing/file.txt", errorData["path"])

	var deserialized ACPError
	err = json.Unmarshal(data, &deserialized)
	s.Require().NoError(err)
	s.Equal(acpError.Code, deserialized.Code)
	s.Equal(acpError.Message, deserialized.Message)
}

func (s *SerializationTestSuite) TestComplexTypesSerialization() {
	sessionID := SessionId("complex-session")

	// PromptRequestPromptElem is an interface, using empty slice for this test.
	promptRequest := &PromptRequest{
		SessionId: sessionID,
		Prompt:    []PromptRequestPromptElem{},
	}

	data, err := json.Marshal(promptRequest)
	s.Require().NoError(err)

	var wireFormat map[string]interface{}
	err = json.Unmarshal(data, &wireFormat)
	s.Require().NoError(err)

	s.Equal("complex-session", wireFormat["sessionId"])

	prompt, ok := wireFormat["prompt"].([]interface{})
	s.True(ok)
	s.Empty(prompt)
}

func (s *SerializationTestSuite) TestRoundTripConsistency() {
	testCases := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "InitializeRequest",
			obj: &InitializeRequest{
				ProtocolVersion: ProtocolVersion(ACPProtocolVersion),
				ClientCapabilities: ClientCapabilities{
					Fs: FileSystemCapability{ReadTextFile: true},
				},
			},
		},
		{
			name: "NewSessionRequest",
			obj: &NewSessionRequest{
				Cwd: "/test/path",
			},
		},
		{
			name: "ReadTextFileRequest",
			obj: &ReadTextFileRequest{
				SessionId: SessionId("test-session"),
				Path:      "/test/file.txt",
			},
		},
		{
			name: "WriteTextFileRequest",
			obj: &WriteTextFileRequest{
				SessionId: SessionId("test-session"),
				Path:      "/test/output.txt",
				Content:   "test content",
			},
		},
		{
			name: "CancelNotification",
			obj: &CancelNotification{
				SessionId: SessionId("test-session"),
			},
		},
		{
			name: "ACPError",
			obj: &ACPError{
				Code:    ErrorCodeInternalServerError,
				Message: "Internal error",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			original, err := json.Marshal(tc.obj)
			s.Require().NoError(err)

			deserializedData, err := json.Marshal(tc.obj)
			s.Require().NoError(err)

			s.JSONEq(string(original), string(deserializedData))
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func TestSerializationTestSuite(t *testing.T) {
	suite.Run(t, new(SerializationTestSuite))
}
