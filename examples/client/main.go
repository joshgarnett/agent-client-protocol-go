package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp"
	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

const (
	minRequiredArgs = 2
	defaultTimeout  = 30 * time.Second
)

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (s stdioReadWriteCloser) Close() error {
	return nil
}

// Global input manager for handling all user input
var inputManager *InputManager

// Test file paths (set by createTestFiles)
var (
	testInputFile   string
	testProjectFile string
	testOutputFile  string
)

// setupAgent creates and starts the agent subprocess.
func setupAgent(ctx context.Context, agentCmd string, agentArgs []string) (stdioReadWriteCloser, *exec.Cmd, error) {
	fmt.Printf("[CLIENT] Starting agent: %s %v\n", agentCmd, agentArgs)

	agentProcess := exec.CommandContext(ctx, agentCmd, agentArgs...)
	agentProcess.Stderr = os.Stderr

	// Set up communication pipes
	agentStdin, err := agentProcess.StdinPipe()
	if err != nil {
		return stdioReadWriteCloser{}, nil, fmt.Errorf("failed to get agent stdin pipe: %w", err)
	}

	agentStdout, err := agentProcess.StdoutPipe()
	if err != nil {
		return stdioReadWriteCloser{}, nil, fmt.Errorf("failed to get agent stdout pipe: %w", err)
	}

	if startErr := agentProcess.Start(); startErr != nil {
		return stdioReadWriteCloser{}, nil, fmt.Errorf("failed to start agent process: %w", startErr)
	}

	fmt.Printf("[CLIENT] Agent process started (PID: %d)\n", agentProcess.Process.Pid)

	stdio := stdioReadWriteCloser{Reader: agentStdout, Writer: agentStdin}
	return stdio, agentProcess, nil
}

// initializeConnection performs ACP initialization and session creation.
func initializeConnection(ctx context.Context, conn *acp.ClientConnection) (api.SessionId, error) {
	fmt.Println("[CLIENT] Establishing connection with agent...")

	// Initialize ACP connection
	initResponse, err := conn.Initialize(ctx, &api.InitializeRequest{
		ProtocolVersion: api.ACPProtocolVersion,
		ClientCapabilities: api.ClientCapabilities{
			Fs: api.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize connection: %w", err)
	}

	fmt.Printf("[CLIENT] Connected to agent (protocol v%d)\n", initResponse.ProtocolVersion)
	fmt.Printf("[CLIENT] Agent capabilities: LoadSession=%t\n", initResponse.AgentCapabilities.LoadSession)

	// Create session
	currentDir, _ := os.Getwd()
	sessionResponse, err := conn.SessionNew(ctx, &api.NewSessionRequest{
		Cwd:        currentDir,
		McpServers: []api.McpServer{},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create new session: %w", err)
	}

	fmt.Printf("[CLIENT] Session created: %s\n", sessionResponse.SessionId)
	return sessionResponse.SessionId, nil
}

// createTestFiles creates test files for the agent to read and demonstrate file operations.
func createTestFiles() error {
	fmt.Println("[CLIENT] Creating test files for agent demonstrations...")

	// Create test input file
	testInput := `# ACP Test Input File

This file demonstrates the Agent Client Protocol file reading capability.

## Project Information
- Name: ACP Go Implementation Test
- Purpose: Demonstrating agent-client file operations
- Created: ` + time.Now().Format(time.RFC3339) + `

## Test Content
This content shows that the agent can successfully read files from the client's filesystem.
The agent will process this file and potentially create new files based on its contents.

## Features Demonstrated
1. File system read operations
2. Content analysis by agents
3. Protocol-compliant file handling
4. Cross-platform path handling

Test data: Lorem ipsum dolor sit amet, consectetur adipiscing elit.
`

	testFile, err := os.CreateTemp("", "acp_test_input_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp test input file: %w", err)
	}
	_ = testFile.Close()
	testInputFile = testFile.Name()

	if writeErr := os.WriteFile(testInputFile, []byte(testInput), 0600); writeErr != nil {
		return fmt.Errorf("failed to write test input file: %w", writeErr)
	}

	// Set output file path for the agent to write to
	outputFile, err := os.CreateTemp("", "acp_agent_output_*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp output file: %w", err)
	}
	_ = outputFile.Close()
	testOutputFile = outputFile.Name()

	// Create project info file
	projectInfo := fmt.Sprintf(`# ACP Go Implementation Project

## Overview
This is an example client-agent interaction using the Agent Client Protocol (ACP).

## Files in this demonstration:
- **%s**: Sample input file for agent to read
- **%s**: This file with project information
- **%s**: Output file created by the agent (after processing)

## Protocol Flow
1. Client advertises file system capabilities (read/write)
2. Agent checks capabilities before making file operations
3. Agent reads input files to understand context
4. Agent requests permission for file writes
5. Agent writes output files with processed information

## Testing
Run the client with: %s

The agent will demonstrate:
- Reading these test files
- Processing the content
- Writing configuration/output files
- Proper permission handling
`, testInputFile, testProjectFile, testOutputFile, "`go run ./examples/client ./examples/agent`")

	projectFile, err := os.CreateTemp("", "acp_project_info_*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp project info file: %w", err)
	}
	_ = projectFile.Close()
	testProjectFile = projectFile.Name()

	if writeErr := os.WriteFile(testProjectFile, []byte(projectInfo), 0600); writeErr != nil {
		return fmt.Errorf("failed to write project info file: %w", writeErr)
	}

	fmt.Printf("[CLIENT] Created test files:\n")
	fmt.Printf("  - %s (%d bytes)\n", testInputFile, len(testInput))
	fmt.Printf("  - %s (%d bytes)\n", testProjectFile, len(projectInfo))
	fmt.Printf("  - %s (ready for agent output)\n", testOutputFile)
	fmt.Println("[CLIENT] Agent can now read these files during operation")

	return nil
}

// cleanupTestFiles removes test files and shows what the agent created.
func cleanupTestFiles() {
	fmt.Println("\n[CLIENT] Checking agent output and cleaning up test files...")

	// Check if agent created output file
	if testOutputFile != "" {
		if content, err := os.ReadFile(testOutputFile); err == nil {
			fmt.Printf("[CLIENT] Agent successfully created output file (%d bytes):\n", len(content))
			fmt.Printf("---\n%s\n---\n", string(content))
		} else {
			fmt.Printf("[CLIENT] Agent output file not found: %v\n", err)
		}
	}

	// Clean up test files
	testFiles := []string{
		testInputFile,
		testProjectFile,
		testOutputFile,
	}

	for _, file := range testFiles {
		if err := os.Remove(file); err != nil {
			fmt.Printf("[CLIENT] Note: Could not remove %s: %v\n", file, err)
		} else {
			fmt.Printf("[CLIENT] Cleaned up: %s\n", file)
		}
	}
}

// runClient performs the main client logic and returns any error.
func runClient() error {
	if len(os.Args) < minRequiredArgs {
		fmt.Printf("Usage: %s <agent_executable> [agent_args...]\n", os.Args[0])
		fmt.Println("Example:")
		fmt.Printf("  %s ../agent/agent\n", os.Args[0])
		fmt.Printf("  go run ./examples/client ./examples/agent/agent\n")
		return errors.New("insufficient arguments")
	}

	agentCmd := os.Args[1]
	agentArgs := os.Args[2:]

	ctx := context.Background()

	// Create test files for agent to read
	if err := createTestFiles(); err != nil {
		return fmt.Errorf("failed to create test files: %w", err)
	}

	// Initialize input manager
	inputManager = NewInputManager()
	inputManager.Start()
	defer inputManager.Stop()

	// Set up handler registry
	registry := acp.NewHandlerRegistry()
	registry.RegisterFsReadTextFileHandler(handleFsReadTextFile)
	registry.RegisterFsWriteTextFileHandler(handleFsWriteTextFile)
	registry.RegisterSessionRequestPermissionHandler(handleSessionRequestPermission)
	registry.RegisterSessionUpdateHandler(handleSessionUpdate)

	// Set up agent connection
	stdio, agentProcess, err := setupAgent(ctx, agentCmd, agentArgs)
	if err != nil {
		return fmt.Errorf("failed to setup agent: %w", err)
	}
	conn, err := acp.NewClientConnectionStdio(ctx, stdio, registry, defaultTimeout)
	if err != nil {
		return fmt.Errorf("failed to create client connection: %w", err)
	}

	// Initialize connection and create session
	sessionID, err := initializeConnection(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
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
	for {
		input, inputErr := inputManager.RequestInput("You: ")
		if inputErr != nil {
			fmt.Printf("[CLIENT] Error reading input: %v\n", inputErr)
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("[CLIENT] Shutting down...")
			break
		}

		if input == "cancel" {
			cancelErr := conn.SessionCancel(ctx, &api.CancelNotification{
				SessionId: sessionID,
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
		var promptResponse *api.PromptResponse
		promptResponse, err = conn.SessionPrompt(ctx, &api.PromptRequest{
			SessionId: sessionID,
			Prompt: []api.PromptRequestPromptElem{
				*api.NewContentBlockText(nil, input),
			},
		})
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

	// Show results and clean up test files
	cleanupTestFiles()

	return nil
}

// Example client demonstrates ACP client implementation with subprocess spawning,
// interactive permission handling, session updates, and file operations.
func main() {
	if err := runClient(); err != nil {
		log.Fatalf("Client error: %v", err)
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
	log.Printf("[CLIENT_DEBUG] handleSessionRequestPermission called")
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

	// For now, automatically allow the first option to avoid deadlock
	// This demonstrates the protocol working - real implementation would get user input
	if len(params.Options) > 0 {
		selectedOption := params.Options[0]
		fmt.Printf("Auto-allowing: %s\n\n", selectedOption.Name)

		response := &api.RequestPermissionResponse{
			Outcome: map[string]interface{}{
				"outcome":  "selected",
				"optionId": selectedOption.OptionId,
			},
		}
		log.Printf("[CLIENT_DEBUG] Returning permission response: %+v", response)
		return response, nil
	}

	// No options available
	fmt.Println("No options available, cancelling")
	return &api.RequestPermissionResponse{
		Outcome: map[string]interface{}{
			"outcome": "cancelled",
		},
	}, nil
}
