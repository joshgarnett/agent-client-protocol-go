package acp

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/joshgarnett/agent-client-protocol-go/util"
)

// TestClient implements a mock client for testing.
type TestClient struct {
	fileContents         *util.SyncMap[string, string]
	writtenFiles         *util.SyncSlice[FileWrite]
	permissionResponses  *util.SyncSlice[api.RequestPermissionResponse]
	sessionNotifications *util.SyncSlice[api.SessionNotification]
	terminals            *util.SyncMap[string, *TestTerminal]
	terminalIDs          int64
	shouldError          *util.SyncMap[string, bool]
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
		fileContents:         util.NewSyncMap[string, string](),
		writtenFiles:         util.NewSyncSlice[FileWrite](),
		permissionResponses:  util.NewSyncSlice[api.RequestPermissionResponse](),
		sessionNotifications: util.NewSyncSlice[api.SessionNotification](),
		terminals:            util.NewSyncMap[string, *TestTerminal](),
		shouldError:          util.NewSyncMap[string, bool](),
	}
}

// AddFileContent adds content for a file path.
func (c *TestClient) AddFileContent(path, content string) {
	c.fileContents.Store(path, content)
}

// GetWrittenFiles returns all files that were written.
func (c *TestClient) GetWrittenFiles() []FileWrite {
	return c.writtenFiles.GetAll()
}

// QueuePermissionResponse queues a permission response.
func (c *TestClient) QueuePermissionResponse(response api.RequestPermissionResponse) {
	c.permissionResponses.Append(response)
}

// GetSessionNotifications returns all received session notifications.
func (c *TestClient) GetSessionNotifications() []api.SessionNotification {
	return c.sessionNotifications.GetAll()
}

// SetShouldError configures whether a method should return an error.
func (c *TestClient) SetShouldError(method string, shouldError bool) {
	c.shouldError.Store(method, shouldError)
}

func (c *TestClient) checkShouldError(method string) bool {
	value, _ := c.shouldError.Load(method)
	return value
}

// Handler implementations.

func (c *TestClient) HandleFsReadTextFile(
	_ context.Context,
	params *api.ReadTextFileRequest,
) (*api.ReadTextFileResponse, error) {
	if c.checkShouldError("fs/read_text_file") {
		return nil, &api.ACPError{Code: api.ErrorCodeNotFound, Message: "File not found"}
	}

	content, exists := c.fileContents.Load(params.Path)

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

	c.writtenFiles.Append(FileWrite{
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

	if c.permissionResponses.Len() == 0 {
		// Default response - cancelled (indicating no specific selection made).
		return &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome": "cancelled",
			},
		}, nil
	}

	// Pop first queued response.
	response, exists := c.permissionResponses.Get(0)
	if !exists {
		return &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome": "cancelled",
			},
		}, nil
	}
	c.permissionResponses.Remove(0)
	return &response, nil
}

func (c *TestClient) HandleSessionUpdate(_ context.Context, params *api.SessionNotification) error {
	if c.checkShouldError("session/update") {
		return &api.ACPError{Code: api.ErrorCodeInternalServerError, Message: "Update failed"}
	}

	c.sessionNotifications.Append(*params)

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

	c.terminals.Store(terminalIDStr, &TestTerminal{
		ID:       terminalIDStr,
		Commands: make([]string, 0),
		Outputs:  make([]string, 0),
	})

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
	c.terminals.Delete(string(params.SessionId))

	return nil
}

func (c *TestClient) HandleTerminalWaitForExit(
	_ context.Context,
	params *api.WaitForTerminalExitRequest,
) (*api.WaitForTerminalExitResponse, error) {
	if c.checkShouldError("terminal/wait_for_exit") {
		return nil, &api.ACPError{Code: api.ErrorCodeNotFound, Message: "Terminal not found"}
	}

	terminal, exists := c.terminals.Load(string(params.SessionId))

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
	if terminal, exists := c.terminals.Load(sessionID); exists {
		terminal.ExitCode = exitCode
	}
}
