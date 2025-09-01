package acp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// TestClient implements a mock client for testing.
type TestClient struct {
	fileContents   map[string]string
	writtenFiles   []FileWrite
	fileContentsMu sync.RWMutex
	writtenFilesMu sync.RWMutex

	permissionResponses []api.RequestPermissionResponse
	permissionMu        sync.Mutex

	sessionNotifications []api.SessionNotification
	sessionMu            sync.RWMutex

	terminals   map[string]*TestTerminal
	terminalsMu sync.RWMutex
	terminalIDs int64

	shouldError   map[string]bool
	shouldErrorMu sync.RWMutex
}

type FileWrite struct {
	Path    string
	Content string
}

type TestTerminal struct {
	ID       string
	Commands []string
	Outputs  []string
	ExitCode *int
}

// NewTestClient creates a new test client.
func NewTestClient() *TestClient {
	return &TestClient{
		fileContents:         make(map[string]string),
		writtenFiles:         make([]FileWrite, 0),
		permissionResponses:  make([]api.RequestPermissionResponse, 0),
		sessionNotifications: make([]api.SessionNotification, 0),
		terminals:            make(map[string]*TestTerminal),
		shouldError:          make(map[string]bool),
	}
}

// AddFileContent adds content for a file path.
func (c *TestClient) AddFileContent(path, content string) {
	c.fileContentsMu.Lock()
	defer c.fileContentsMu.Unlock()
	c.fileContents[path] = content
}

// GetWrittenFiles returns all files that were written.
func (c *TestClient) GetWrittenFiles() []FileWrite {
	c.writtenFilesMu.RLock()
	defer c.writtenFilesMu.RUnlock()
	result := make([]FileWrite, len(c.writtenFiles))
	copy(result, c.writtenFiles)
	return result
}

// QueuePermissionResponse queues a permission response.
func (c *TestClient) QueuePermissionResponse(response api.RequestPermissionResponse) {
	c.permissionMu.Lock()
	defer c.permissionMu.Unlock()
	c.permissionResponses = append(c.permissionResponses, response)
}

// GetSessionNotifications returns all received session notifications.
func (c *TestClient) GetSessionNotifications() []api.SessionNotification {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	result := make([]api.SessionNotification, len(c.sessionNotifications))
	copy(result, c.sessionNotifications)
	return result
}

// SetShouldError configures whether a method should return an error.
func (c *TestClient) SetShouldError(method string, shouldError bool) {
	c.shouldErrorMu.Lock()
	defer c.shouldErrorMu.Unlock()
	c.shouldError[method] = shouldError
}

func (c *TestClient) checkShouldError(method string) bool {
	c.shouldErrorMu.RLock()
	defer c.shouldErrorMu.RUnlock()
	return c.shouldError[method]
}

// Handler implementations.

func (c *TestClient) HandleFsReadTextFile(
	_ context.Context,
	params *api.ReadTextFileRequest,
) (*api.ReadTextFileResponse, error) {
	if c.checkShouldError("fs/read_text_file") {
		return nil, &api.ACPError{Code: api.ErrorCodeNotFound, Message: "File not found"}
	}

	c.fileContentsMu.RLock()
	content, exists := c.fileContents[params.Path]
	c.fileContentsMu.RUnlock()

	if !exists {
		content = "default content"
	}

	return &api.ReadTextFileResponse{
		Content: content,
	}, nil
}

func (c *TestClient) HandleFsWriteTextFile(_ context.Context, params *api.WriteTextFileRequest) error {
	if c.checkShouldError("fs/write_text_file") {
		return &api.ACPError{Code: api.ErrorCodeForbidden, Message: "Write not allowed"}
	}

	c.writtenFilesMu.Lock()
	defer c.writtenFilesMu.Unlock()
	c.writtenFiles = append(c.writtenFiles, FileWrite{
		Path:    params.Path,
		Content: params.Content,
	})

	return nil
}

func (c *TestClient) HandleRequestPermission(
	_ context.Context,
	_ *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	if c.checkShouldError("session/request_permission") {
		return nil, &api.ACPError{Code: api.ErrorCodeUnauthorized, Message: "Permission denied"}
	}

	c.permissionMu.Lock()
	defer c.permissionMu.Unlock()

	if len(c.permissionResponses) == 0 {
		// Default response - cancelled (indicating no specific selection made).
		return &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome": "cancelled",
			},
		}, nil
	}

	// Pop first queued response.
	response := c.permissionResponses[0]
	c.permissionResponses = c.permissionResponses[1:]
	return &response, nil
}

func (c *TestClient) HandleSessionUpdate(_ context.Context, params *api.SessionNotification) error {
	if c.checkShouldError("session/update") {
		return &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Update failed"}
	}

	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	c.sessionNotifications = append(c.sessionNotifications, *params)

	return nil
}

func (c *TestClient) HandleTerminalCreate(
	_ context.Context,
	_ *api.CreateTerminalRequest,
) (*api.CreateTerminalResponse, error) {
	if c.checkShouldError("terminal/create") {
		return nil, &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Terminal creation failed"}
	}

	terminalID := atomic.AddInt64(&c.terminalIDs, 1)
	terminalIDStr := fmt.Sprintf("term-%d", terminalID)

	c.terminalsMu.Lock()
	defer c.terminalsMu.Unlock()
	c.terminals[terminalIDStr] = &TestTerminal{
		ID:       terminalIDStr,
		Commands: make([]string, 0),
		Outputs:  make([]string, 0),
	}

	return &api.CreateTerminalResponse{
		TerminalId: terminalIDStr,
	}, nil
}

func (c *TestClient) HandleTerminalOutput(_ context.Context, _ *api.TerminalOutputRequest) error {
	if c.checkShouldError("terminal/output") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Terminal not found"}
	}

	// For testing, we just log the output.
	return nil
}

func (c *TestClient) HandleTerminalRelease(_ context.Context, params *api.ReleaseTerminalRequest) error {
	if c.checkShouldError("terminal/release") {
		return &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Terminal not found"}
	}

	// Remove terminal.
	c.terminalsMu.Lock()
	defer c.terminalsMu.Unlock()
	delete(c.terminals, string(params.SessionId))

	return nil
}

func (c *TestClient) HandleTerminalWaitForExit(
	_ context.Context,
	params *api.WaitForTerminalExitRequest,
) (*api.WaitForTerminalExitResponse, error) {
	if c.checkShouldError("terminal/wait_for_exit") {
		return nil, &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Terminal not found"}
	}

	c.terminalsMu.RLock()
	terminal, exists := c.terminals[string(params.SessionId)]
	c.terminalsMu.RUnlock()

	if !exists {
		return nil, &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Terminal not found"}
	}

	exitCode := 0
	if terminal.ExitCode != nil {
		exitCode = *terminal.ExitCode
	}

	return &api.WaitForTerminalExitResponse{
		ExitCode: &exitCode,
	}, nil
}

func NewPermissionCancelledOutcome() api.RequestPermissionResponseOutcome {
	return map[string]interface{}{
		"outcome": "cancelled",
	}
}

func NewPermissionSelectedOutcome(optionID string) api.RequestPermissionResponseOutcome {
	return map[string]interface{}{
		"outcome":  "selected",
		"optionId": optionID,
	}
}

const (
	StopReasonEndTurn         = "end_turn"
	StopReasonMaxTokens       = "max_tokens"
	StopReasonMaxTurnRequests = "max_turn_requests"
	StopReasonRefusal         = "refusal"
)

func NewPromptResponseEndTurn() *api.PromptResponse {
	return &api.PromptResponse{
		StopReason: StopReasonEndTurn,
	}
}

func NewPromptResponseMaxTokens() *api.PromptResponse {
	return &api.PromptResponse{
		StopReason: StopReasonMaxTokens,
	}
}

func NewPromptResponseRefusal() *api.PromptResponse {
	return &api.PromptResponse{
		StopReason: StopReasonRefusal,
	}
}

func (c *TestClient) SetTerminalExitCode(sessionID string, exitCode *int) {
	c.terminalsMu.Lock()
	defer c.terminalsMu.Unlock()
	if terminal, exists := c.terminals[sessionID]; exists {
		terminal.ExitCode = exitCode
	}
}
