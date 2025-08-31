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
)

func main() {
    ctx := context.Background()
    
    // Create handler registry
    registry := acp.NewHandlerRegistry()
    registry.RegisterInitializeHandler(handleInitialize)
    registry.RegisterSessionNewHandler(handleSessionNew)
    
    // Create stdio connection
    stdio := &stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
    conn := acp.NewAgentConnectionStdio(ctx, stdio, registry.Handler())
    
    log.Println("Agent started")
    conn.Wait()
}

func handleInitialize(ctx context.Context, params *acp.InitializeRequest) (*acp.InitializeResponse, error) {
    return &acp.InitializeResponse{
        AgentCapabilities: acp.AgentCapabilities{
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
)

func main() {
    ctx := context.Background()
    
    // Create handler registry
    registry := acp.NewHandlerRegistry()
    registry.RegisterFsReadTextFileHandler(handleFileRead)
    
    // Create stdio connection
    stdio := &stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout}
    conn := acp.NewClientConnectionStdio(ctx, stdio, registry.Handler())
    
    log.Println("Client started")
    conn.Wait()
}

func handleFileRead(ctx context.Context, params *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
    // Handle file read request from agent
    return &acp.ReadTextFileResponse{}, nil
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
registry.RegisterInitializeHandler(func(ctx context.Context, params *acp.InitializeRequest) (*acp.InitializeResponse, error) {
    // Handle initialize request
    return response, nil
})

// Register notification handlers (fire-and-forget)
registry.RegisterSessionCancelHandler(func(ctx context.Context, params *acp.SessionCancelParams) error {
    // Handle session cancel notification
    return nil
})
```


## Requirements

- Go 1.25 or later

## Protocol Support

This implementation supports Agent Client Protocol version 1 and includes:

### Agent Methods (Agent -> Client)
- `initialize`: Initialize connection with capabilities
- `authenticate`: Authenticate with the client  
- `session/new`: Create a new session
- `session/load`: Load an existing session
- `session/prompt`: Send a prompt to the client
- `session/cancel`: Cancel session operations

### Client Methods (Client -> Agent)  
- `fs/read_text_file`: Read text file contents
- `fs/write_text_file`: Write text file contents
- `session/request_permission`: Request permissions
- `session/update`: Update session state
- `terminal/create`: Create terminal session
- `terminal/output`: Send terminal output
- `terminal/release`: Release terminal session
- `terminal/wait_for_exit`: Wait for terminal exit

## Examples

See the `examples/` directory for complete working examples:
- `examples/agent/`: Example agent implementation
- `examples/client/`: Example client implementation

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, architecture details, and guidelines.

## License

This project is licensed under the MIT License.
