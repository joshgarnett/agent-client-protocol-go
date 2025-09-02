package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/joshgarnett/agent-client-protocol-go/util"
)

const (
	// Timing constants for simulation delays in milliseconds.
	initialThinkingDelay = 800
	fileReadDelay        = 600
	progressUpdateDelay  = 500
	finalProcessDelay    = 400
	toolExecutionDelay   = 1000

	// Content display limits.
	maxDisplayContentLength = 200
)

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (s stdioReadWriteCloser) Close() error {
	return nil
}

// discoverTestFiles finds ACP test files created by the client.
func discoverTestFiles() ([]string, error) {
	tempDir := os.TempDir()
	patterns := []string{
		"acp_test_input_*.txt",
		"acp_project_info_*.md",
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	return files, nil
}

// discoverOutputFilePath finds the output file path created by the client.
func discoverOutputFilePath() (string, error) {
	tempDir := os.TempDir()
	pattern := "acp_agent_output_*.json"

	matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
	if err != nil {
		return "", fmt.Errorf("failed to glob output file pattern: %w", err)
	}

	if len(matches) == 0 {
		// Return a default temp file path if no existing one is found
		return filepath.Join(tempDir, "acp_agent_output.json"), nil
	}

	return matches[0], nil
}

// activePrompts tracks ongoing prompts for cancellation support.
var activePrompts = util.NewSyncMap[string, context.CancelFunc]()

// Global agent connection for handlers that need it
var agentConn *acp.AgentConnection

// Global client capabilities received during initialization
var clientCapabilities *api.ClientCapabilities

// Example agent demonstrates ACP agent implementation with session management,
// tool calls, permission handling, and cancellation support.
func main() {
	ctx := context.Background()

	registry := acp.NewHandlerRegistry()

	// Register all handlers (including the one that needs the connection)
	registry.RegisterInitializeHandler(handleInitialize)
	registry.RegisterAuthenticateHandler(handleAuthenticate)
	registry.RegisterSessionNewHandler(handleSessionNew)
	registry.RegisterSessionPromptHandler(handleSessionPrompt)
	registry.RegisterSessionCancelHandler(handleSessionCancel)

	stdio := stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}

	// Create connection with all handlers registered
	conn, err := acp.NewAgentConnectionStdio(
		ctx,
		stdio,
		registry,
		10*time.Second, //nolint:mnd // 10 second timeout is reasonable for agent operations
	)
	if err != nil {
		log.Fatalf("Failed to create agent connection: %v", err)
	}

	// Store connection globally for use by handlers
	agentConn = conn

	log.Printf("Example Agent started (PID: %d), waiting for client connection...\n", os.Getpid())
	if waitErr := conn.Wait(); waitErr != nil {
		log.Printf("Connection closed: %v\n", waitErr)
	} else {
		log.Println("Connection closed gracefully")
	}

	log.Println("Example Agent stopped")
}

// handleInitialize handles the initialize request from the client.
func handleInitialize(_ context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
	log.Printf("[INIT] Received initialize request from client (protocol v%d)\n", params.ProtocolVersion)
	log.Printf("[INIT] Client capabilities: %+v\n", params.ClientCapabilities)

	// Store client capabilities globally for use in file operations
	clientCapabilities = &params.ClientCapabilities

	// Log what file operations the client supports
	if clientCapabilities.Fs.ReadTextFile {
		log.Println("[INIT] Client supports file reading")
	}
	if clientCapabilities.Fs.WriteTextFile {
		log.Println("[INIT] Client supports file writing")
	}

	// Return our capabilities to the client.
	response := &api.InitializeResponse{
		ProtocolVersion: api.ACPProtocolVersion,
		AgentCapabilities: api.AgentCapabilities{
			LoadSession:        true, // This agent can restore previous sessions
			PromptCapabilities: api.PromptCapabilities{},
		},
		AuthMethods: []api.AuthMethod{}, // No authentication required
	}

	log.Println("[INIT] Sent initialize response - connection established")
	return response, nil
}

// handleAuthenticate handles authenticate requests.
func handleAuthenticate(_ context.Context, params *api.AuthenticateRequest) error {
	log.Printf("[AUTH] Authentication requested with method: %v\n", params.MethodId)
	// This example doesn't implement authentication - just return success
	log.Println("[AUTH] Authentication successful (no-op)")
	return nil
}

// handleSessionNew handles session/new requests.
func handleSessionNew(_ context.Context, params *api.NewSessionRequest) (*api.NewSessionResponse, error) {
	log.Printf("[SESSION] Creating new session in directory: %s\n", params.Cwd)
	if len(params.McpServers) > 0 {
		log.Printf("[SESSION] MCP servers configured: %d\n", len(params.McpServers))
		for i, server := range params.McpServers {
			log.Printf("[SESSION] MCP Server %d: %s\n", i+1, server.Name)
		}
	}

	// Generate a session ID - use UUID in production
	sessionID := fmt.Sprintf("sess_%d", time.Now().Unix())

	response := &api.NewSessionResponse{
		SessionId: api.SessionId(sessionID),
	}

	log.Printf("[SESSION] Created new session: %s\n", sessionID)
	return response, nil
}

// handleSessionPrompt handles session/prompt requests and demonstrates agent workflow.
func handleSessionPrompt(_ context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	log.Printf("[PROMPT] Starting prompt processing for session: %s\n", params.SessionId)
	log.Printf("[PROMPT] Received %d content blocks\n", len(params.Prompt))

	// Use global connection for session updates
	if agentConn == nil {
		log.Println("[ERROR] Agent connection not available")
		return &api.PromptResponse{StopReason: api.StopReasonRefusal}, errors.New("agent connection not available")
	}

	// With the new jsonrpc2 library, the connection is fully bidirectional,
	// allowing handlers to make blocking calls that are safely multiplexed
	// over the same connection without causing a deadlock.
	// This matches the TypeScript reference implementation pattern.

	// Create a cancellable context for this prompt
	promptCtx, promptCancel := context.WithCancel(context.Background())

	// Store the cancel function for this session
	activePrompts.Store(string(params.SessionId), promptCancel)

	defer func() {
		// Clean up when done
		activePrompts.Delete(string(params.SessionId))
	}()

	// Now we can call agent methods directly from the handler!
	stopReason, err := simulateAgentTurn(promptCtx, agentConn, params)
	if err != nil {
		log.Printf("[PROMPT] Error during agent simulation: %v\n", err)
		return &api.PromptResponse{StopReason: api.StopReasonRefusal}, err
	}

	log.Printf("[PROMPT] Agent turn completed with stop reason: %s\n", stopReason)
	return &api.PromptResponse{StopReason: api.StopReasonEndTurn}, nil
}

// handleSessionCancel handles session cancellation requests.
func handleSessionCancel(_ context.Context, params *api.CancelNotification) error {
	log.Printf("[CANCEL] Cancellation requested for session: %s\n", params.SessionId)

	if cancel, exists := activePrompts.LoadAndDelete(string(params.SessionId)); exists {
		cancel()
		log.Printf("[CANCEL] Successfully cancelled session: %s\n", params.SessionId)
	} else {
		log.Printf("[CANCEL] No active prompt found for session: %s\n", params.SessionId)
	}

	return nil
}

// simulateAgentTurn demonstrates a multi-step agent workflow with tool calls and permission requests.
func simulateAgentTurn(
	ctx context.Context,
	conn *acp.AgentConnection,
	params *api.PromptRequest,
) (api.StopReason, error) {
	// Step 1: Send initial thinking/analysis message
	message := "I'll help you with that. Let me start by analyzing the request and reading some files to understand the current situation."
	err := sendAgentMessage(ctx, conn, params.SessionId, message)
	if err != nil {
		return api.StopReasonRefusal, err
	}

	if simErr := simulateProcessing(ctx, initialThinkingDelay); simErr != nil {
		return api.StopReasonCancelled, simErr
	}

	// Step 2: Actual file reading operation
	toolErr := performFileReadOperation(ctx, conn, params.SessionId, "call_1")
	if toolErr != nil {
		if ctx.Err() == context.Canceled {
			return api.StopReasonCancelled, nil
		}
		return api.StopReasonRefusal, toolErr
	}

	if simErr := simulateProcessing(ctx, fileReadDelay); simErr != nil {
		return api.StopReasonCancelled, simErr
	}

	// Step 3: Send progress update
	progressMsg := " Based on my analysis, I need to make some changes. Let me modify a configuration file."
	err = sendAgentMessage(ctx, conn, params.SessionId, progressMsg)
	if err != nil {
		return api.StopReasonRefusal, err
	}

	if simErr := simulateProcessing(ctx, progressUpdateDelay); simErr != nil {
		return api.StopReasonCancelled, simErr
	}

	// Step 4: Actual file writing operation (with permission)
	toolErr = performFileWriteOperation(ctx, conn, params.SessionId, "call_2")
	if toolErr != nil {
		if ctx.Err() == context.Canceled {
			return api.StopReasonCancelled, nil
		}
		return api.StopReasonRefusal, toolErr
	}

	if simErr := simulateProcessing(ctx, finalProcessDelay); simErr != nil {
		return api.StopReasonCancelled, simErr
	}

	// Step 5: Send final completion message
	completionMsg := " Perfect! I've successfully completed the requested changes. The configuration has been updated and the project is ready."
	err = sendAgentMessage(ctx, conn, params.SessionId, completionMsg)
	if err != nil {
		return api.StopReasonRefusal, err
	}

	return api.StopReasonEndTurn, nil
}

// sendAgentMessage sends an agent message chunk to the client.
func sendAgentMessage(ctx context.Context, conn *acp.AgentConnection, sessionID api.SessionId, text string) error {
	textContent := api.NewContentBlockText(nil, text)
	agentMessageUpdate := api.NewSessionUpdateAgentMessageChunk(textContent)

	err := conn.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    agentMessageUpdate,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to send agent message: %v\n", err)
		return err
	}
	return nil
}

// simulateProcessing adds realistic delays with cancellation support.
func simulateProcessing(ctx context.Context, delayMs int) error {
	select {
	case <-time.After(time.Duration(delayMs) * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// performFileReadOperation demonstrates actual file reading using the agent connection.
func performFileReadOperation(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID string,
) error {
	log.Printf("[FILE] Starting file read operation (%s)\n", toolCallID)

	// Step 1: Check if client supports file reading
	if clientCapabilities == nil || !clientCapabilities.Fs.ReadTextFile {
		log.Printf("[FILE] Client does not support file reading, skipping operation\n")
		return sendToolCallComplete(
			ctx, conn, sessionID, toolCallID,
			"Reading project files (skipped)",
			"Client does not support file reading", "")
	}

	// Step 2: Send tool call start notification
	if err := sendToolCallStart(ctx, conn, sessionID, toolCallID, "Reading project files", api.ToolKindRead); err != nil {
		return err
	}

	// Step 3: Discover and read test files that the client created
	filesToRead, err := discoverTestFiles()
	if err != nil {
		log.Printf("[FILE] Failed to discover test files: %v\n", err)
		// Continue with empty list
		filesToRead = []string{}
	}

	var filesRead []string
	var totalContent strings.Builder

	for _, filePath := range filesToRead {
		log.Printf("[FILE] Attempting to read: %s\n", filePath)

		response, readErr := conn.FsReadTextFile(ctx, &api.ReadTextFileRequest{
			SessionId: sessionID,
			Path:      filePath,
		})

		if readErr != nil {
			log.Printf("[FILE] Failed to read %s: %v\n", filePath, readErr)
			continue // Skip files that can't be read
		}

		log.Printf("[FILE] Successfully read %s (%d bytes)\n", filePath, len(response.Content))
		filesRead = append(filesRead, filePath)

		// Truncate content for display but keep full content for processing
		displayContent := response.Content
		if len(displayContent) > maxDisplayContentLength {
			displayContent = displayContent[:maxDisplayContentLength] + "... (truncated)"
		}
		totalContent.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", filePath, displayContent))
	}

	// Step 4: Send tool call completion with results
	var resultMessage string
	if len(filesRead) > 0 {
		resultMessage = fmt.Sprintf("Successfully read %d files: %v", len(filesRead), filesRead)
	} else {
		resultMessage = "No test files could be read - client may not have created them yet"
	}

	return sendToolCallComplete(
		ctx,
		conn,
		sessionID,
		toolCallID,
		"Reading project files",
		resultMessage,
		totalContent.String(),
	)
}

// performFileWriteOperation demonstrates actual file writing with permission request.
func performFileWriteOperation(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID string,
) error {
	log.Printf("[FILE] Starting file write operation (%s)\n", toolCallID)

	// Step 1: Check if client supports file writing
	if clientCapabilities == nil || !clientCapabilities.Fs.WriteTextFile {
		log.Printf("[FILE] Client does not support file writing, skipping operation\n")
		return sendToolCallComplete(
			ctx,
			conn,
			sessionID,
			toolCallID,
			"Writing configuration file (skipped)",
			"Client does not support file writing",
			"",
		)
	}

	// Step 2: Send tool call start notification
	if err := sendToolCallStart(ctx, conn, sessionID, toolCallID, "Writing configuration file", api.ToolKindEdit); err != nil {
		return err
	}

	// Step 3: Request permission for the file write operation
	granted, err := requestFileWritePermission(ctx, conn, sessionID, toolCallID)
	if err != nil {
		return fmt.Errorf("permission request failed: %w", err)
	}

	if !granted {
		log.Printf("[FILE] Permission denied for file write operation\n")
		return sendToolCallComplete(
			ctx,
			conn,
			sessionID,
			toolCallID,
			"Writing configuration file (skipped)",
			"Permission denied",
			"",
		)
	}

	// Step 4: Write the configuration file
	configPath, err := discoverOutputFilePath()
	if err != nil {
		log.Printf("[FILE] Failed to discover output file path: %v\n", err)
		return sendToolCallComplete(
			ctx,
			conn,
			sessionID,
			toolCallID,
			"Writing configuration file (failed)",
			"Failed to determine output path",
			"",
		)
	}
	configContent := fmt.Sprintf(`{
  "agent_id": "example-agent",
  "session_id": "%s",
  "timestamp": "%s",
  "version": "1.0.0",
  "operation": "file_write_test",
  "settings": {
    "debug": true,
    "max_retries": 3,
    "test_mode": true
  },
  "capabilities_received": {
    "fs_read": %t,
    "fs_write": %t
  }
}`, sessionID, time.Now().Format(time.RFC3339), clientCapabilities.Fs.ReadTextFile, clientCapabilities.Fs.WriteTextFile)

	log.Printf("[FILE] Writing configuration to: %s\n", configPath)

	err = conn.FsWriteTextFile(ctx, &api.WriteTextFileRequest{
		SessionId: sessionID,
		Path:      configPath,
		Content:   configContent,
	})

	if err != nil {
		log.Printf("[FILE] Failed to write %s: %v\n", configPath, err)
		return sendToolCallComplete(
			ctx,
			conn,
			sessionID,
			toolCallID,
			"Writing configuration file (failed)",
			fmt.Sprintf("Write failed: %v", err),
			"",
		)
	}

	log.Printf("[FILE] Successfully wrote configuration file: %s\n", configPath)
	return sendToolCallComplete(
		ctx,
		conn,
		sessionID,
		toolCallID,
		"Writing configuration file (completed)",
		fmt.Sprintf("Successfully wrote %s (%d bytes)", configPath, len(configContent)),
		configContent[:minInt(maxDisplayContentLength, len(configContent))],
	)
}

// sendToolCallStart sends a tool call start notification.
func sendToolCallStart(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID, title string,
	kind api.ToolKind,
) error {
	toolStatus := api.ToolCallStatusPending
	location := api.ToolCallLocation{Path: "/project/"}

	toolCall := api.NewSessionUpdateToolCall(
		[]interface{}{}, // initial content empty
		&kind,
		[]interface{}{location},
		map[string]interface{}{"operation": title},
		nil, // rawOutput - nil initially
		&toolStatus,
		title,
		(*api.ToolCallId)(&toolCallID),
	)

	return conn.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    toolCall,
	})
}

// sendToolCallComplete sends a tool call completion notification.
func sendToolCallComplete(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID, title, message, content string,
) error {
	completedStatus := api.ToolCallStatusCompleted
	var contentBlocks []interface{}

	if message != "" {
		textBlock := api.NewContentBlockText(nil, message)
		contentBlocks = append(contentBlocks, *textBlock)
	}

	toolCallUpdate := api.NewSessionUpdateToolCallUpdate(
		contentBlocks,
		nil, // kind
		nil, // locations
		nil, // rawInput
		map[string]interface{}{"success": true, "content": content}, // rawOutput
		&completedStatus,
		title+" (completed)",
		(*api.ToolCallId)(&toolCallID),
	)

	return conn.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    toolCallUpdate,
	})
}

// requestFileWritePermission requests permission for file write operations.
func requestFileWritePermission(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID string,
) (bool, error) {
	log.Printf("[PERMISSION] Requesting write permission for tool call: %s\n", toolCallID)

	// Create permission options
	allowOption := api.PermissionOption{
		Kind:     api.PermissionOptionKindAllowOnce,
		Name:     "Allow file write",
		OptionId: api.PermissionOptionId("allow"),
	}

	rejectOption := api.PermissionOption{
		Kind:     api.PermissionOptionKindRejectOnce,
		Name:     "Skip file write",
		OptionId: api.PermissionOptionId("reject"),
	}

	// Create tool call for permission request
	configPath, err := discoverOutputFilePath()
	if err != nil {
		log.Printf("[FILE] Failed to discover output file path for permission request: %v\n", err)
		configPath = "/tmp/acp_agent_output.json" // fallback
	}
	toolCall := api.ToolCallUpdate{
		Kind:       api.ToolKindEdit,
		Status:     api.ToolCallStatusPending,
		Title:      stringPtr("Writing configuration file"),
		ToolCallId: api.ToolCallId(toolCallID),
		Locations:  []api.ToolCallLocation{{Path: configPath}},
		RawInput: map[string]interface{}{
			"path":      configPath,
			"operation": "write configuration",
			"test_mode": true,
		},
	}

	// Make the permission request
	permissionRequest := &api.RequestPermissionRequest{
		SessionId: sessionID,
		ToolCall:  toolCall,
		Options:   []api.PermissionOption{allowOption, rejectOption},
	}

	response, err := conn.SessionRequestPermission(ctx, permissionRequest)
	if err != nil {
		return false, fmt.Errorf("permission request call failed: %w", err)
	}

	// Parse the response
	outcomeMap, ok := response.Outcome.(map[string]interface{})
	if !ok {
		return false, errors.New("invalid permission response format")
	}

	outcome, ok := outcomeMap["outcome"].(string)
	if !ok {
		return false, errors.New("invalid permission response format")
	}

	if outcome == "cancelled" {
		log.Printf("[PERMISSION] Request was cancelled\n")
		return false, nil
	}

	if outcome == "selected" {
		optionID, optionOk := outcomeMap["optionId"].(string)
		if !optionOk {
			return false, errors.New("invalid permission option ID format")
		}

		granted := optionID == "allow"
		log.Printf("[PERMISSION] Permission %s (option: %s)\n",
			map[bool]string{true: "granted", false: "denied"}[granted], optionID)
		return granted, nil
	}

	return false, fmt.Errorf("unexpected permission outcome: %s", outcome)
}

// Helper functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtr(s string) *string {
	return &s
}
