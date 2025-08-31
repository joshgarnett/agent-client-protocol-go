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

	// Create a handler registry for incoming requests from the agent.
	registry := acp.NewHandlerRegistry()

	// Register handlers for the methods the agent might call on us.
	registry.RegisterFsReadTextFileHandler(handleFsReadTextFile)
	registry.RegisterFsWriteTextFileHandler(handleFsWriteTextFile)

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
func handleFsReadTextFile(_ context.Context, params *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	log.Printf("Agent requested to read file: %+v", params)

	// In a real implementation, you'd read the file and return its contents.
	response := &acp.ReadTextFileResponse{
		// Add file contents.
	}

	return response, nil
}

// handleFsWriteTextFile handles file write requests from the agent.
func handleFsWriteTextFile(_ context.Context, params *acp.WriteTextFileRequest) error {
	log.Printf("Agent requested to write file: %+v", params)

	// In a real implementation, you'd write the file.
	return nil
}
