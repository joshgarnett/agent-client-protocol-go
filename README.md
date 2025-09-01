# Agent Client Protocol - Go Implementation

A Go library for implementing the Agent Client Protocol (ACP), providing type-safe client and agent connections with automatic code generation from the official JSON schema.

## Overview

The Agent Client Protocol enables communication between AI agents and clients through a standardized JSON-RPC 2.0 interface. This Go implementation provides:

- **Type-safe API**: Generated Go types from the official ACP JSON schema
- **Connection management**: Both agent and client connection types with stdio transport
- **Handler registration**: Type-safe method and notification handlers  
- **Error handling**: Protocol-specific error types and constants
- **Code generation**: Automated type and constant generation from upstream schema

## Installation

```bash
go get github.com/joshgarnett/agent-client-protocol-go
```

## Quick Start

### Agent Implementation

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/joshgarnett/agent-client-protocol-go/acp"
    "github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

func main() {
    ctx := context.Background()
    
    // Create handler registry
    registry := acp.NewHandlerRegistry()
    registry.RegisterInitializeHandler(handleInitialize)
    registry.RegisterSessionNewHandler(handleSessionNew)
    registry.RegisterSessionPromptHandler(handleSessionPrompt)
    
    // Create stdio connection
    stdio := &stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
    conn := acp.NewAgentConnectionStdio(ctx, stdio, registry.Handler())
    
    log.Println("Agent started")
    conn.Wait()
}

func handleInitialize(ctx context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
    return &api.InitializeResponse{
        ProtocolVersion: api.ACPProtocolVersion,
        AgentCapabilities: api.AgentCapabilities{
            LoadSession: true,
        },
    }, nil
}
```

### Client Implementation

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/joshgarnett/agent-client-protocol-go/acp"
    "github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

func main() {
    ctx := context.Background()
    
    // Create handler registry
    registry := acp.NewHandlerRegistry()
    registry.RegisterFsReadTextFileHandler(handleFileRead)
    registry.RegisterSessionUpdateHandler(handleSessionUpdate)
    
    // Create stdio connection
    stdio := &stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
    conn := acp.NewClientConnectionStdio(ctx, stdio, registry.Handler())
    
    log.Println("Client started")
    conn.Wait()
}

func handleFileRead(ctx context.Context, params *api.ReadTextFileRequest) (*api.ReadTextFileResponse, error) {
    // Handle file read request from agent
    return &api.ReadTextFileResponse{}, nil
}
```

## Architecture

### Connection Types

- **AgentConnection**: Represents an agent's connection to a client
- **ClientConnection**: Represents a client's connection to an agent

Both connection types support:
- JSON-RPC 2.0 method calls and notifications
- stdio transport (other transports can be added)
- Type-safe method helpers
- Connection lifecycle management

### Handler Registry

The `HandlerRegistry` provides type-safe registration of method and notification handlers:

```go
registry := acp.NewHandlerRegistry()

// Register method handlers (request/response)
registry.RegisterInitializeHandler(func(ctx context.Context, params *api.InitializeRequest) (*api.InitializeResponse, error) {
    // Handle initialize request
    return response, nil
})

// Register notification handlers (fire-and-forget)
registry.RegisterSessionCancelHandler(func(ctx context.Context, params *api.CancelNotification) error {
    // Handle session cancel notification
    return nil
})
```


## Requirements

- Go 1.25 or later

## Protocol Support

This implementation supports Agent Client Protocol version 1 with the following methods:

### Agent Methods (Client -> Agent)
- `initialize`: Initialize connection with capabilities
- `authenticate`: Authenticate with the agent
- `session/new`: Create a new session (with MCP server support)
- `session/load`: Load an existing session (if agent supports `loadSession` capability)
- `session/prompt`: Send a prompt to the agent

### Agent Notifications (Client -> Agent)
- `session/cancel`: Cancel ongoing operations

### Client Methods (Agent -> Client)
- `session/request_permission`: Request user authorization for tool calls
- `fs/read_text_file`: Read text file contents (requires `fs.readTextFile` capability)
- `fs/write_text_file`: Write text file contents (requires `fs.writeTextFile` capability)
- `terminal/create`: Create terminal session
- `terminal/output`: Send terminal output  
- `terminal/release`: Release terminal session
- `terminal/wait_for_exit`: Wait for terminal exit

### Client Notifications (Agent -> Client)
- `session/update`: Progress updates during prompt processing (agent message chunks, tool calls, plans)

## Examples

See the `examples/` directory for complete working examples:
- `examples/agent/`: Example agent implementation with session management, tool calls, and permission handling
- `examples/client/`: Example client implementation with subprocess management and interactive communication

### Running the Examples

```bash
# Build the examples
go build -o examples/agent/agent ./examples/agent
go build -o examples/client/client ./examples/client

# Run interactive client (spawns agent automatically)
go run ./examples/client ./examples/agent/agent

# Or using built binaries
./examples/client/client ./examples/agent/agent
```

The client will start an interactive session where you can chat with the agent, see tool calls in action, and approve permissions for file operations.

For detailed documentation, protocol flows, and advanced usage patterns, see [examples/README.md](examples/README.md).

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, architecture details, and guidelines.

## License

This project is licensed under the MIT License.
