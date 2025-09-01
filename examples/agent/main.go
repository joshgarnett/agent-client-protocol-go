package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (s stdioReadWriteCloser) Close() error {
	// For stdio, we don't actually close anything.
	return nil
}

func main() {
	ctx := context.Background()

	// Create a handler registry for incoming requests from the client.
	registry := acp.NewHandlerRegistry()

	// Register handlers for the methods we support.
	registry.RegisterInitializeHandler(handleInitialize)
	registry.RegisterSessionNewHandler(handleSessionNew)
	registry.RegisterSessionPromptHandler(handleSessionPrompt)

	// For demo purposes, we'll use a stdio connection.
	// In practice, you might use other transports like HTTP or WebSocket.
	stdio := stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
	conn := acp.NewAgentConnectionStdio(ctx, stdio, registry.Handler())

	log.Println("Agent started, waiting for client connection...")

	// Wait for the connection to close.
	if err := conn.Wait(); err != nil {
		log.Fatalf("Connection error: %v", err)
	}

	log.Println("Agent stopped")
}

// handleInitialize handles the initialize request from the client.
func handleInitialize(_ context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
	log.Printf("Received initialize request: %+v", params)

	// Return our capabilities.
	response := &api.InitializeResponse{
		AgentCapabilities: api.AgentCapabilities{
			LoadSession:        true,
			PromptCapabilities: api.PromptCapabilities{
				// Add prompt capabilities based on what this agent supports.
			},
		},
	}

	log.Println("Sent initialize response")
	return response, nil
}

// handleSessionNew handles session/new requests.
func handleSessionNew(_ context.Context, params *api.NewSessionRequest) (*api.NewSessionResponse, error) {
	log.Printf("Received session/new request: %+v", params)

	// Create a new session ID (in practice, you'd generate a proper unique ID).
	sessionID := "session-123"

	response := &api.NewSessionResponse{
		SessionId: api.SessionId(sessionID),
	}

	log.Printf("Created new session: %s", sessionID)
	return response, nil
}

// handleSessionPrompt handles session/prompt requests.
func handleSessionPrompt(ctx context.Context, params *api.PromptRequest) (*api.PromptResponse, error) {
	log.Printf("Received session/prompt request for session: %v", params.SessionId)

	// In a real implementation, you would:
	// 1. Send the prompt to your LLM
	// 2. Process any tool calls requested by the LLM
	// 3. Send session/update notifications for progress
	// 4. Return the final response

	// Example: Simulate agent processing with session updates using type-safe generated types
	if conn, ok := ctx.Value("agent_connection").(*acp.AgentConnection); ok {
		// Create a text content block
		textContent := api.NewContentBlockText(nil, "I'll analyze your request...")

		// Send agent message chunk using type-safe generated union
		agentMessageUpdate := api.NewSessionUpdateAgentMessageChunk(textContent)
		err := conn.SendSessionUpdate(ctx, &api.SessionNotification{
			SessionId: params.SessionId,
			Update:    agentMessageUpdate,
		})
		if err != nil {
			log.Printf("Error sending session update: %v", err)
		}

		// Simulate tool call with type-safe enums
		toolKind := api.ToolKindThink
		toolStatus := api.ToolCallStatusCompleted
		toolCallUpdate := api.NewSessionUpdateToolCall(
			[]interface{}{"Processing request..."}, // content
			&toolKind,                              // kind
			nil,                                    // locations
			"analyze request",                      // rawInput
			"request analyzed",                     // rawOutput
			&toolStatus,                            // status
			"Processing request",                   // title
			nil,                                    // toolCallId
		)

		err = conn.SendSessionUpdate(ctx, &api.SessionNotification{
			SessionId: params.SessionId,
			Update:    toolCallUpdate,
		})
		if err != nil {
			log.Printf("Error sending tool call update: %v", err)
		}
	}

	// Use type-safe enum for stop reason
	response := &api.PromptResponse{
		StopReason: api.StopReasonEndTurn,
	}

	log.Println("Sent prompt response")
	return response, nil
}
