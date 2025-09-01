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

	// Create a handler registry for incoming requests from the agent.
	registry := acp.NewHandlerRegistry()

	// Register handlers for the methods the agent might call on us.
	registry.RegisterFsReadTextFileHandler(handleFsReadTextFile)
	registry.RegisterFsWriteTextFileHandler(handleFsWriteTextFile)
	registry.RegisterSessionRequestPermissionHandler(handleSessionRequestPermission)
	registry.RegisterSessionUpdateHandler(handleSessionUpdate)

	// Create a connection to the agent.
	stdio := stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
	conn := acp.NewClientConnectionStdio(ctx, stdio, registry.Handler())

	log.Println("Client started, connecting to agent...")

	// Wait for the connection to close.
	// In practice, the client would handle incoming requests from the agent.
	// via the registered handlers and also make outgoing requests to the agent.
	// using the connection methods like FsReadTextFile, TerminalCreate, etc.

	log.Println("Client running, waiting for agent communication...")

	if err := conn.Wait(); err != nil {
		log.Printf("Connection error: %v", err)
	}

	log.Println("Client stopped")
}

// handleFsReadTextFile handles file read requests from the agent.
func handleFsReadTextFile(_ context.Context, params *api.ReadTextFileRequest) (*api.ReadTextFileResponse, error) {
	log.Printf("Agent requested to read file: %+v", params)

	// In a real implementation, you'd read the file and return its contents.
	response := &api.ReadTextFileResponse{
		// Add file contents.
	}

	return response, nil
}

// handleFsWriteTextFile handles file write requests from the agent.
func handleFsWriteTextFile(_ context.Context, params *api.WriteTextFileRequest) error {
	log.Printf("Agent requested to write file: %+v", params)

	// In a real implementation, you'd write the file.
	return nil
}

// handleSessionRequestPermission handles permission requests from the agent.
func handleSessionRequestPermission(
	_ context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	log.Printf("Agent requested permission: %+v", params)

	// In a real implementation, you'd show this to the user and get their decision.
	// For this example, we'll automatically allow the request using type-safe generated types.

	// Find an "allow once" option from the provided options
	var allowOnceOptionID string
	for _, option := range params.Options {
		if option.Kind == api.PermissionOptionKindAllowOnce {
			allowOnceOptionID = string(option.OptionId)
			break
		}
	}

	// If no allow once option found, use the first option as fallback
	if allowOnceOptionID == "" && len(params.Options) > 0 {
		allowOnceOptionID = string(params.Options[0].OptionId)
	}

	response := &api.RequestPermissionResponse{
		Outcome: map[string]interface{}{
			"outcome":  "selected",
			"optionId": allowOnceOptionID,
		},
	}

	log.Printf("Permission granted with option ID: %s", allowOnceOptionID)
	return response, nil
}
