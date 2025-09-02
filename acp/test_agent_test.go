package acp

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/joshgarnett/agent-client-protocol-go/util"
)

// TestAgent implements a mock agent for testing.
type TestAgent struct {
	sessions              *util.SyncMap[string, *TestSession]
	sessionCounter        int64
	promptsReceived       *util.SyncSlice[PromptReceived]
	cancellationsReceived *util.SyncSlice[string]
	authenticated         *util.AtomicValue[bool]
	shouldError           *util.SyncMap[string, bool]
	capabilities          api.AgentCapabilities
	authMethods           []api.AuthMethod
}

type TestSession struct {
	ID  string
	CWD string
}

type PromptReceived struct {
	SessionID string
	Prompt    []api.PromptRequestPromptElem
}

// NewTestAgent creates a new test agent.
func NewTestAgent() *TestAgent {
	return &TestAgent{
		sessions:              util.NewSyncMap[string, *TestSession](),
		promptsReceived:       util.NewSyncSlice[PromptReceived](),
		cancellationsReceived: util.NewSyncSlice[string](),
		authenticated:         util.NewAtomicValue[bool](false),
		shouldError:           util.NewSyncMap[string, bool](),
		capabilities: api.AgentCapabilities{
			LoadSession:        true,
			PromptCapabilities: api.PromptCapabilities{},
		},
		authMethods: []api.AuthMethod{
			{
				Id:          api.AuthMethodId("test-auth"),
				Name:        "Test Authentication",
				Description: StringPtr("Test authentication method"),
			},
		},
	}
}

// GetSessions returns all created sessions.
func (a *TestAgent) GetSessions() map[string]*TestSession {
	return a.sessions.GetAll()
}

// GetPromptsReceived returns all prompts received.
func (a *TestAgent) GetPromptsReceived() []PromptReceived {
	return a.promptsReceived.GetAll()
}

// GetCancellationsReceived returns all cancellations received.
func (a *TestAgent) GetCancellationsReceived() []string {
	return a.cancellationsReceived.GetAll()
}

// SetShouldError configures whether a method should return an error.
func (a *TestAgent) SetShouldError(method string, shouldError bool) {
	a.shouldError.Store(method, shouldError)
}

// SetCapabilities sets the agent capabilities.
func (a *TestAgent) SetCapabilities(capabilities api.AgentCapabilities) {
	a.capabilities = capabilities
}

// SetAuthMethods sets the authentication methods.
func (a *TestAgent) SetAuthMethods(methods []api.AuthMethod) {
	a.authMethods = methods
}

func (a *TestAgent) checkShouldError(method string) bool {
	value, _ := a.shouldError.Load(method)
	return value
}

func (a *TestAgent) HandleInitialize(
	_ context.Context,
	params *api.InitializeRequest,
) (*api.InitializeResponse, error) {
	if a.checkShouldError("initialize") {
		return nil, &api.ACPError{Code: api.ErrorCodeInitializationError, Message: "Initialization failed"}
	}

	return &api.InitializeResponse{
		ProtocolVersion:   params.ProtocolVersion,
		AgentCapabilities: a.capabilities,
		AuthMethods:       a.authMethods,
	}, nil
}

func (a *TestAgent) HandleAuthenticate(_ context.Context, _ *api.AuthenticateRequest) error {
	if a.checkShouldError("authenticate") {
		return &api.ACPError{Code: api.ErrorCodeUnauthorized, Message: "Authentication failed"}
	}

	a.authenticated.Store(true)

	return nil
}

func (a *TestAgent) HandleSessionNew(
	_ context.Context,
	params *api.NewSessionRequest,
) (*api.NewSessionResponse, error) {
	if a.checkShouldError("session/new") {
		return nil, &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Session creation failed"}
	}

	sessionID := fmt.Sprintf("session-%d", atomic.AddInt64(&a.sessionCounter, 1))

	session := &TestSession{
		ID:  sessionID,
		CWD: params.Cwd,
	}

	a.sessions.Store(sessionID, session)

	return &api.NewSessionResponse{
		SessionId: api.SessionId(sessionID),
	}, nil
}

func (a *TestAgent) HandleSessionLoad(_ context.Context, params *api.LoadSessionRequest) error {
	if a.checkShouldError("session/load") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	// For testing, we just verify the session exists.
	_, exists := a.sessions.Load(string(params.SessionId))

	if !exists {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	return nil
}

func (a *TestAgent) HandleSessionPrompt(_ context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	if a.checkShouldError("session/prompt") {
		return nil, &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Prompt processing failed"}
	}

	a.promptsReceived.Append(PromptReceived{
		SessionID: string(params.SessionId),
		Prompt:    params.Prompt,
	})

	return &api.PromptResponse{
		StopReason: "end_turn",
	}, nil
}

func (a *TestAgent) HandleSessionCancel(_ context.Context, params *api.CancelNotification) error {
	if a.checkShouldError("session/cancel") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	a.cancellationsReceived.Append(string(params.SessionId))

	return nil
}

// IsAuthenticated returns whether the agent is authenticated.
func (a *TestAgent) IsAuthenticated() bool {
	return a.authenticated.Load()
}
