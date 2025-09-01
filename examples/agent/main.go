package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

const (
	// Timing constants for simulation delays in milliseconds.
	initialThinkingDelay = 800
	fileReadDelay        = 600
	progressUpdateDelay  = 500
	finalProcessDelay    = 400
	toolExecutionDelay   = 1000
)

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (s stdioReadWriteCloser) Close() error {
	return nil
}

// activePrompts tracks ongoing prompts for cancellation support.
var activePrompts = make(map[string]context.CancelFunc)
var promptsMutex sync.RWMutex

// Example agent demonstrates ACP agent implementation with session management,
// tool calls, permission handling, and cancellation support.
func main() {
	ctx := context.Background()

	registry := acp.NewHandlerRegistry()

	registry.RegisterInitializeHandler(handleInitialize)
	registry.RegisterAuthenticateHandler(handleAuthenticate)
	registry.RegisterSessionNewHandler(handleSessionNew)
	registry.RegisterSessionPromptHandler(handleSessionPrompt)
	registry.RegisterSessionCancelHandler(handleSessionCancel)

	stdio := stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
	conn := acp.NewAgentConnectionStdio(ctx, stdio, registry.Handler())

	log.Printf("Example Agent started (PID: %d), waiting for client connection...\n", os.Getpid())
	if err := conn.Wait(); err != nil {
		log.Printf("Connection closed: %v\n", err)
	} else {
		log.Println("Connection closed gracefully")
	}

	log.Println("Example Agent stopped")
}

// handleInitialize handles the initialize request from the client.
func handleInitialize(_ context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
	log.Printf("[INIT] Received initialize request from client (protocol v%d)\n", params.ProtocolVersion)
	log.Printf("[INIT] Client capabilities: %+v\n", params.ClientCapabilities)

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
func handleSessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	log.Printf("[PROMPT] Starting prompt processing for session: %s\n", params.SessionId)
	log.Printf("[PROMPT] Received %d content blocks\n", len(params.Prompt))

	// Set up cancellation support for this prompt
	promptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	promptsMutex.Lock()
	activePrompts[string(params.SessionId)] = cancel
	promptsMutex.Unlock()

	defer func() {
		promptsMutex.Lock()
		delete(activePrompts, string(params.SessionId))
		promptsMutex.Unlock()
	}()

	// Get agent connection from context to send session updates
	conn, ok := ctx.Value("agent_connection").(*acp.AgentConnection)
	if !ok {
		log.Println("[ERROR] Agent connection not found in context")
		return &api.PromptResponse{StopReason: api.StopReasonRefusal}, errors.New("agent connection not available")
	}

	// Run the agent simulation workflow
	stopReason, err := simulateAgentTurn(promptCtx, conn, params)
	if err != nil {
		log.Printf("[PROMPT] Error during agent simulation: %v\n", err)
		if promptCtx.Err() == context.Canceled {
			return &api.PromptResponse{StopReason: api.StopReasonCancelled}, nil
		}
		return &api.PromptResponse{StopReason: api.StopReasonRefusal}, err
	}

	log.Printf("[PROMPT] Completed prompt processing with stop reason: %s\n", stopReason)
	return &api.PromptResponse{StopReason: stopReason}, nil
}

// handleSessionCancel handles session cancellation requests.
func handleSessionCancel(_ context.Context, params *api.CancelNotification) error {
	log.Printf("[CANCEL] Cancellation requested for session: %s\n", params.SessionId)

	promptsMutex.Lock()
	if cancel, exists := activePrompts[string(params.SessionId)]; exists {
		cancel()
		delete(activePrompts, string(params.SessionId))
		log.Printf("[CANCEL] Successfully cancelled session: %s\n", params.SessionId)
	} else {
		log.Printf("[CANCEL] No active prompt found for session: %s\n", params.SessionId)
	}
	promptsMutex.Unlock()

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

	// Step 2: Tool call that doesn't require permission
	toolErr := simulateToolCall(
		ctx, conn, params.SessionId, "call_1",
		"Reading project files", api.ToolKindRead, false,
	)
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

	// Step 4: Tool call that requires user permission
	toolErr = simulateToolCall(
		ctx, conn, params.SessionId, "call_2",
		"Modifying configuration file", api.ToolKindEdit, true,
	)
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

// simulateToolCall demonstrates the tool call lifecycle with optional permission requests.
func simulateToolCall(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID, title string,
	kind api.ToolKind,
	needsPermission bool,
) error {
	// Step 1: Send initial tool call notification
	toolStatus := api.ToolCallStatusPending
	location := api.ToolCallLocation{Path: "/project/config.json"}

	toolCall := api.NewSessionUpdateToolCall(
		[]interface{}{}, // initial content empty
		&kind,
		[]interface{}{location}, // convert to []interface{}
		map[string]interface{}{"path": "/project/config.json", "operation": title},
		nil, // rawOutput - nil initially
		&toolStatus,
		title,
		(*api.ToolCallId)(&toolCallID), // convert to ToolCallId type
	)

	err := conn.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    toolCall,
	})
	if err != nil {
		return fmt.Errorf("failed to send tool call: %w", err)
	}

	log.Printf("[TOOL] Started tool call '%s' (%s)\n", title, toolCallID)

	// Step 2: If permission is needed, request it
	if needsPermission {
		permissionGranted, permErr := requestPermission(ctx, conn, sessionID, toolCallID, title, kind, location)
		if permErr != nil {
			return fmt.Errorf("permission request failed: %w", permErr)
		}

		if !permissionGranted {
			// Send tool call update showing it was skipped
			skippedStatus := api.ToolCallStatusCompleted
			skippedUpdate := api.NewSessionUpdateToolCallUpdate(
				[]interface{}{}, // content
				nil,             // kind
				nil,             // locations
				nil,             // rawInput
				map[string]interface{}{"skipped": true, "reason": "permission denied"}, // rawOutput
				&skippedStatus,                 // status
				title+" (skipped)",             // title
				(*api.ToolCallId)(&toolCallID), // toolCallID
			)

			return conn.SendSessionUpdate(ctx, &api.SessionNotification{
				SessionId: sessionID,
				Update:    skippedUpdate,
			})
		}
	}

	// Step 3: Simulate tool execution
	if simErr := simulateProcessing(ctx, toolExecutionDelay); simErr != nil {
		return simErr
	}

	// Step 4: Send completion update
	completedStatus := api.ToolCallStatusCompleted
	successText := api.NewContentBlockText(nil, "Operation completed successfully")

	toolCallUpdate := api.NewSessionUpdateToolCallUpdate(
		[]interface{}{*successText}, // content
		nil,                         // kind
		nil,                         // locations
		nil,                         // rawInput
		map[string]interface{}{"success": true, "message": "File updated successfully"}, // rawOutput
		&completedStatus,               // status
		title+" (completed)",           // title
		(*api.ToolCallId)(&toolCallID), // toolCallID
	)

	err = conn.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    toolCallUpdate,
	})
	if err != nil {
		return fmt.Errorf("failed to send tool call update: %w", err)
	}

	log.Printf("[TOOL] Completed tool call '%s' (%s)\n", title, toolCallID)
	return nil
}

// requestPermission requests user permission for sensitive operations.
func requestPermission(
	ctx context.Context,
	conn *acp.AgentConnection,
	sessionID api.SessionId,
	toolCallID, title string,
	kind api.ToolKind,
	location api.ToolCallLocation,
) (bool, error) {
	log.Printf("[PERMISSION] Requesting permission for: %s\n", title)

	// Create the tool call structure for the permission request
	toolStatus := api.ToolCallStatusPending
	toolCallForPermission := api.ToolCallUpdate{
		Kind:       &kind,
		Status:     toolStatus,
		Title:      &title,
		ToolCallId: api.ToolCallId(toolCallID),
		Locations:  []api.ToolCallLocation{location},
		RawInput: map[string]interface{}{
			"path":      location.Path,
			"operation": title,
		},
	}

	// Create permission options
	allowOption := api.PermissionOption{
		Kind:     api.PermissionOptionKindAllowOnce,
		Name:     "Allow this change",
		OptionId: api.PermissionOptionId("allow"),
	}

	rejectOption := api.PermissionOption{
		Kind:     api.PermissionOptionKindRejectOnce,
		Name:     "Skip this change",
		OptionId: api.PermissionOptionId("reject"),
	}

	// Make the permission request
	permissionRequest := &api.RequestPermissionRequest{
		SessionId: sessionID,
		ToolCall:  toolCallForPermission,
		Options:   []api.PermissionOption{allowOption, rejectOption},
	}

	var response api.RequestPermissionResponse
	err := conn.Call(ctx, api.MethodSessionRequestPermission, permissionRequest, &response)
	if err != nil {
		return false, fmt.Errorf("permission request call failed: %w", err)
	}

	// Parse the response - Outcome is an interface{} that should be a map
	outcomeMap, ok := response.Outcome.(map[string]interface{})
	if !ok {
		return false, errors.New("invalid permission response format: outcome is not a map")
	}

	outcome, ok := outcomeMap["outcome"].(string)
	if !ok {
		return false, errors.New("invalid permission response format: outcome field missing or not string")
	}

	if outcome == "cancelled" {
		log.Printf("[PERMISSION] Permission request was cancelled\n")
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

// simulateProcessing adds realistic delays with cancellation support.
func simulateProcessing(ctx context.Context, delayMs int) error {
	select {
	case <-time.After(time.Duration(delayMs) * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
