package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

const minRequiredArgs = 2

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (s stdioReadWriteCloser) Close() error {
	return nil
}

// Example client demonstrates ACP client implementation with subprocess spawning,
// interactive permission handling, session updates, and file operations.
func main() {
	if len(os.Args) < minRequiredArgs {
		fmt.Printf("Usage: %s <agent_executable> [agent_args...]\n", os.Args[0])
		fmt.Println("Example:")
		fmt.Printf("  %s ../agent/agent\n", os.Args[0])
		fmt.Printf("  go run ./examples/client ./examples/agent/agent\n")
		os.Exit(1)
	}

	agentCmd := os.Args[1]
	agentArgs := os.Args[2:]

	ctx := context.Background()

	registry := acp.NewHandlerRegistry()

	registry.RegisterFsReadTextFileHandler(handleFsReadTextFile)
	registry.RegisterFsWriteTextFileHandler(handleFsWriteTextFile)
	registry.RegisterSessionRequestPermissionHandler(handleSessionRequestPermission)
	registry.RegisterSessionUpdateHandler(handleSessionUpdate)

	// Start agent subprocess
	fmt.Printf("[CLIENT] Starting agent: %s %v\n", agentCmd, agentArgs)

	agentProcess := exec.CommandContext(ctx, agentCmd, agentArgs...)
	agentProcess.Stderr = os.Stderr

	// Set up communication pipes
	agentStdin, err := agentProcess.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get agent stdin pipe: %v", err)
	}

	agentStdout, err := agentProcess.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get agent stdout pipe: %v", err)
	}

	if startErr := agentProcess.Start(); startErr != nil {
		log.Fatalf("Failed to start agent process: %v", startErr)
	}

	fmt.Printf("[CLIENT] Agent process started (PID: %d)\n", agentProcess.Process.Pid)

	// Connect to agent via stdio
	stdio := stdioReadWriteCloser{Reader: agentStdout, Writer: agentStdin}
	conn := acp.NewClientConnectionStdio(ctx, stdio, registry.Handler())

	fmt.Println("[CLIENT] Establishing connection with agent...")

	// Initialize ACP connection
	var initResponse api.InitializeResponse
	err = conn.Call(ctx, api.MethodInitialize, &api.InitializeRequest{
		ProtocolVersion: api.ACPProtocolVersion,
		ClientCapabilities: api.ClientCapabilities{
			Fs: api.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
		},
	}, &initResponse)
	if err != nil {
		log.Fatalf("Failed to initialize connection: %v", err)
	}

	fmt.Printf("[CLIENT] Connected to agent (protocol v%d)\n", initResponse.ProtocolVersion)
	fmt.Printf("[CLIENT] Agent capabilities: LoadSession=%t\n", initResponse.AgentCapabilities.LoadSession)

	// Create session
	currentDir, _ := os.Getwd()
	var sessionResponse api.NewSessionResponse
	err = conn.Call(ctx, api.MethodSessionNew, &api.NewSessionRequest{
		Cwd:        currentDir,
		McpServers: []api.McpServer{},
	}, &sessionResponse)
	if err != nil {
		log.Fatalf("Failed to create new session: %v", err)
	}

	fmt.Printf("[CLIENT] Created session: %s\n", sessionResponse.SessionId)
	fmt.Println()
	fmt.Println("=====================================")
	fmt.Println("Agent Client Protocol Demo")
	fmt.Println("=====================================")
	fmt.Println("Type messages to send to the agent.")
	fmt.Println("Type 'quit' or 'exit' to stop.")
	fmt.Println("Type 'cancel' to cancel the current agent operation.")
	fmt.Println("=====================================")
	fmt.Println()

	// Interactive prompt loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("[CLIENT] Shutting down...")
			break
		}

		if input == "cancel" {
			cancelErr := conn.Notify(ctx, api.MethodSessionCancel, &api.CancelNotification{
				SessionId: sessionResponse.SessionId,
			})
			if cancelErr != nil {
				fmt.Printf("[CLIENT] Error cancelling session: %v\n", cancelErr)
			} else {
				fmt.Println("[CLIENT] Cancellation request sent")
			}
			continue
		}

		// Send prompt to agent
		fmt.Println("Agent:", "")
		var promptResponse api.PromptResponse
		err = conn.Call(ctx, api.MethodSessionPrompt, &api.PromptRequest{
			SessionId: sessionResponse.SessionId,
			Prompt: []api.PromptRequestPromptElem{
				*api.NewContentBlockText(nil, input),
			},
		}, &promptResponse)
		if err != nil {
			fmt.Printf("\n[CLIENT] Error sending prompt: %v\n", err)
			continue
		}

		fmt.Printf("\n[CLIENT] Agent completed with stop reason: %v\n\n", promptResponse.StopReason)
	}

	// Clean up
	fmt.Println("[CLIENT] Closing connection...")
	if closeErr := conn.Close(); closeErr != nil {
		fmt.Printf("[CLIENT] Error closing connection: %v\n", closeErr)
	}

	// Wait for agent to finish
	if waitErr := agentProcess.Wait(); waitErr != nil {
		fmt.Printf("[CLIENT] Agent process exited with error: %v\n", waitErr)
	} else {
		fmt.Println("[CLIENT] Agent process exited successfully")
	}
}

// handleFsReadTextFile handles file read requests from the agent.
func handleFsReadTextFile(_ context.Context, params *api.ReadTextFileRequest) (*api.ReadTextFileResponse, error) {
	fmt.Printf("[FILE] Agent requested to read file: %s\n", params.Path)

	// Handle relative paths
	path := params.Path
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			path = filepath.Join(cwd, path)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[FILE] Error reading file %s: %v\n", params.Path, err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fmt.Printf("[FILE] Successfully read file %s (%d bytes)\n", params.Path, len(content))

	return &api.ReadTextFileResponse{
		Content: string(content),
	}, nil
}

// handleFsWriteTextFile handles file write requests from the agent.
func handleFsWriteTextFile(_ context.Context, params *api.WriteTextFileRequest) error {
	fmt.Printf("[FILE] Agent requested to write file: %s (%d bytes)\n", params.Path, len(params.Content))

	// Handle relative paths
	path := params.Path
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			path = filepath.Join(cwd, path)
		}
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		fmt.Printf("[FILE] Error creating directory for %s: %v\n", params.Path, err)
		return fmt.Errorf("failed to create directory: %w", err)
	}
	err := os.WriteFile(path, []byte(params.Content), 0600)
	if err != nil {
		fmt.Printf("[FILE] Error writing file %s: %v\n", params.Path, err)
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("[FILE] Successfully wrote file %s\n", params.Path)
	return nil
}

// handleSessionRequestPermission handles permission requests from the agent.
func handleSessionRequestPermission(
	_ context.Context,
	params *api.RequestPermissionRequest,
) (*api.RequestPermissionResponse, error) {
	fmt.Printf("\n[PERMISSION] Agent requested permission for: %v\n", params.ToolCall.Title)

	// ToolCall.Kind is an interface{}, need to handle it carefully
	if kindVal := params.ToolCall.Kind; kindVal != nil {
		if kindStr, ok := kindVal.(string); ok {
			fmt.Printf("    Tool kind: %s\n", kindStr)
		} else {
			fmt.Printf("    Tool kind: %v\n", kindVal)
		}
	}

	if len(params.ToolCall.Locations) > 0 {
		fmt.Println("    Affected locations:")
		for _, loc := range params.ToolCall.Locations {
			fmt.Printf("      - %s\n", loc.Path)
		}
	}

	fmt.Printf("    Raw input: %+v\n", params.ToolCall.RawInput)
	fmt.Println()
	fmt.Println("Available options:")

	for i, option := range params.Options {
		fmt.Printf("  %d. %s (%s)\n", i+1, option.Name, option.Kind)
	}

	// Get user's choice
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Choose an option (1-" + strconv.Itoa(len(params.Options)) + "): ")
		if !scanner.Scan() {
			// EOF - default to first option
			break
		}

		choice := strings.TrimSpace(scanner.Text())
		if choice == "" {
			continue
		}

		choiceNum, err := strconv.Atoi(choice)
		if err != nil || choiceNum < 1 || choiceNum > len(params.Options) {
			fmt.Println("Invalid choice. Please try again.")
			continue
		}

		selectedOption := params.Options[choiceNum-1]
		fmt.Printf("Selected: %s\n\n", selectedOption.Name)

		return &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome":  "selected",
				"optionId": selectedOption.OptionId,
			},
		}, nil
	}

	// If we get here, there was an EOF or error - default to first option
	if len(params.Options) > 0 {
		selectedOption := params.Options[0]
		fmt.Printf("Defaulting to: %s\n\n", selectedOption.Name)

		return &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome":  "selected",
				"optionId": selectedOption.OptionId,
			},
		}, nil
	}

	// No options available
	return &api.RequestPermissionResponse{
		Outcome: map[string]interface{}{
			"outcome": "cancelled",
		},
	}, nil
}
