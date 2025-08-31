package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
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
func handleInitialize(_ context.Context, params *acp.InitializeRequest) (*acp.InitializeResponse, error) {
	log.Printf("Received initialize request: %+v", params)

	// Return our capabilities.
	response := &acp.InitializeResponse{
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession:        true,
			PromptCapabilities: acp.PromptCapabilities{
				// Add prompt capabilities based on what this agent supports.
			},
		},
	}

	log.Println("Sent initialize response")
	return response, nil
}

// handleSessionNew handles session/new requests.
func handleSessionNew(_ context.Context, params *acp.NewSessionRequest) (*acp.NewSessionResponse, error) {
	log.Printf("Received session/new request: %+v", params)

	// Create a new session ID (in practice, you'd generate a proper unique ID).
	sessionID := "session-123"

	response := &acp.NewSessionResponse{
		SessionId: acp.SessionId(sessionID),
	}

	log.Printf("Created new session: %s", sessionID)
	return response, nil
}

// handleSessionPrompt handles session/prompt requests.
func handleSessionPrompt(_ context.Context, params *acp.PromptRequest) (*acp.PromptResponse, error) {
	log.Printf("Received session/prompt request for session: %v", params.SessionId)

	// For this example, we'll just echo back a simple response.
	// In a real agent, you'd process the prompt and generate a meaningful response.

	response := &acp.PromptResponse{
		// Add response content based on the prompt.
		// This would typically involve AI processing, tool usage, etc.
	}

	log.Println("Sent prompt response")
	return response, nil
}
