package acp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// TestAgent implements a mock agent for testing.
type TestAgent struct {
	sessions       map[string]*TestSession
	sessionsMu     sync.RWMutex
	sessionCounter int64

	promptsReceived []PromptReceived
	promptsMu       sync.RWMutex

	cancellationsReceived []string
	cancellationsMu       sync.RWMutex

	authenticated bool
	authMu        sync.RWMutex

	shouldError   map[string]bool
	shouldErrorMu sync.RWMutex

	capabilities api.AgentCapabilities
	authMethods  []api.AuthMethod
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
		sessions:              make(map[string]*TestSession),
		promptsReceived:       make([]PromptReceived, 0),
		cancellationsReceived: make([]string, 0),
		shouldError:           make(map[string]bool),
		capabilities: api.AgentCapabilities{
			LoadSession:        true,
			PromptCapabilities: api.PromptCapabilities{},
		},
		authMethods: []api.AuthMethod{
			{
				Id:          api.AuthMethodId("test-auth"),
				Name:        "Test Authentication",
				Description: stringPtr("Test authentication method"),
			},
		},
	}
}

// GetSessions returns all created sessions.
func (a *TestAgent) GetSessions() map[string]*TestSession {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()
	result := make(map[string]*TestSession)
	for k, v := range a.sessions {
		result[k] = v
	}
	return result
}

// GetPromptsReceived returns all prompts received.
func (a *TestAgent) GetPromptsReceived() []PromptReceived {
	a.promptsMu.RLock()
	defer a.promptsMu.RUnlock()
	result := make([]PromptReceived, len(a.promptsReceived))
	copy(result, a.promptsReceived)
	return result
}

// GetCancellationsReceived returns all cancellations received.
func (a *TestAgent) GetCancellationsReceived() []string {
	a.cancellationsMu.RLock()
	defer a.cancellationsMu.RUnlock()
	result := make([]string, len(a.cancellationsReceived))
	copy(result, a.cancellationsReceived)
	return result
}

// SetShouldError configures whether a method should return an error.
func (a *TestAgent) SetShouldError(method string, shouldError bool) {
	a.shouldErrorMu.Lock()
	defer a.shouldErrorMu.Unlock()
	a.shouldError[method] = shouldError
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
	a.shouldErrorMu.RLock()
	defer a.shouldErrorMu.RUnlock()
	return a.shouldError[method]
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

	a.authMu.Lock()
	defer a.authMu.Unlock()
	a.authenticated = true

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

	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	a.sessions[sessionID] = session

	return &api.NewSessionResponse{
		SessionId: api.SessionId(sessionID),
	}, nil
}

func (a *TestAgent) HandleSessionLoad(_ context.Context, params *api.LoadSessionRequest) error {
	if a.checkShouldError("session/load") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	// For testing, we just verify the session exists.
	a.sessionsMu.RLock()
	_, exists := a.sessions[string(params.SessionId)]
	a.sessionsMu.RUnlock()

	if !exists {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	return nil
}

func (a *TestAgent) HandleSessionPrompt(_ context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	if a.checkShouldError("session/prompt") {
		return nil, &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Prompt processing failed"}
	}

	a.promptsMu.Lock()
	a.promptsReceived = append(a.promptsReceived, PromptReceived{
		SessionID: string(params.SessionId),
		Prompt:    params.Prompt,
	})
	a.promptsMu.Unlock()

	return &api.PromptResponse{
		StopReason: "end_turn",
	}, nil
}

func (a *TestAgent) HandleSessionCancel(_ context.Context, params *api.CancelNotification) error {
	if a.checkShouldError("session/cancel") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Session not found"}
	}

	a.cancellationsMu.Lock()
	a.cancellationsReceived = append(a.cancellationsReceived, string(params.SessionId))
	a.cancellationsMu.Unlock()

	return nil
}

// IsAuthenticated returns whether the agent is authenticated.
func (a *TestAgent) IsAuthenticated() bool {
	a.authMu.RLock()
	defer a.authMu.RUnlock()
	return a.authenticated
}

func stringPtr(s string) *string {
	return &s
}
