# Contributing to agent-client-protocol-go

Thank you for your interest in contributing to the Agent Client Protocol Go implementation!

## Development Setup

### Prerequisites

- Go 1.25 or later
- Make
- curl (for schema fetching)

### Initial Setup

1. Clone the repository:
```bash
git clone https://github.com/joshgarnett/agent-client-protocol-go.git
cd agent-client-protocol-go
```

2. Install required tools and dependencies:
```bash
make setup
```

3. Generate types and constants from the upstream schema:
```bash
make generate
```

4. Build all packages:
```bash
make build
```

## Code Generation

This library uses automated code generation to stay synchronized with the official ACP specification:

### Generated Files

- `acp/api/types_generated.go`: Go types generated from the upstream JSON schema
- `acp/api/constants_generated.go`: Method constants generated from meta.json

### Generation Process

1. **Schema Fetching**: Downloads `schema.json` and `meta.json` from the official ACP repository
2. **Type Generation**: Uses `go-jsonschema` CLI tool to generate Go types from the JSON schema  
3. **Constants Generation**: Uses our custom CLI tool to generate method constants from meta.json

### Make Targets

```bash
make setup        # Install required tools and dependencies
make fetch-schema # Download latest schema files from upstream
make generate     # Generate types and constants (includes fetch-schema)
make build        # Build all packages
make test         # Run tests
make fmt          # Format code
make vet          # Run static analysis
make lint         # Run linters
make check        # Run test + fmt + vet + lint
make examples     # Build example binaries
```

## Code Quality

All code must pass the following quality checks:

```bash
make check       # Run test + fmt + vet + lint
```

Or individually:
```bash
go build ./...    # Build all packages
go test ./...     # Run all tests  
go fmt ./...      # Format code
go vet ./...      # Static analysis
golangci-lint run # Linting
```

## Project Structure

```
agent-client-protocol-go/
├── Makefile             # Build automation
├── go.mod               # Go module definition
├── README.md            # User documentation
├── CONTRIBUTING.md      # This file
├── .gitignore           # Git ignore rules
├── schema/              # Downloaded schemas (gitignored)
│   ├── schema.json      # From upstream ACP
│   └── meta.json        # From upstream ACP  
├── acp/                 # Core library package
│   ├── api/             # Generated API types and constants
│   │   ├── types_generated.go      # Generated types
│   │   └── constants_generated.go  # Generated constants
│   ├── agent.go         # Agent connection implementation
│   ├── client.go        # Client connection implementation
│   ├── handlers.go      # Handler registration system
│   └── errors.go        # Error types and constants
├── examples/            # Usage examples
│   ├── README.md        # Detailed examples documentation
│   ├── agent/           # Example agent implementation
│   │   └── main.go      # Agent with session management and tool calls
│   └── client/          # Example client implementation
│       ├── main.go      # Interactive client with subprocess management
│       └── session_handler.go # Session update handling
└── cmd/generate/        # Code generation CLI tool
    ├── main.go          # CLI entry point
    └── cmd/
        ├── root.go      # Root command
        └── constants.go # Constants generation command
```

## Architecture Decisions

### Schema-First Approach

We generate Go types FROM the JSON schema rather than generating the schema from Go types. This ensures we stay synchronized with the official protocol specification.

### Type Safety

The library provides type-safe APIs wherever possible:
- Generated types from JSON schema
- Type-safe handler registration
- Method-specific connection helpers

### JSON-RPC Foundation


- Proven JSON-RPC 2.0 implementation
- Standard error codes and handling  
- Flexible transport support

### Handler Registry Pattern

The `HandlerRegistry` provides a clean way to register type-safe handlers for incoming requests and notifications.

## Adding New Features

### Adding New Transport Types

1. Implement `jsonrpc2.ObjectStream` for your transport
2. Add constructor functions to connection types
3. Add examples showing usage
4. Update documentation

### Adding New Handler Types

1. Add typed handler registration methods to `HandlerRegistry`
2. Follow existing patterns for parameter unmarshaling
3. Add examples and documentation
4. Ensure error handling follows protocol requirements

## Testing

### Unit Tests

Write unit tests for all non-generated code:
- Handler registration and execution
- Connection lifecycle
- Error handling
- Custom logic

### Integration Tests  

Test cross-language compatibility:
- Go agent with reference client implementations
- Go client with reference agent implementations
- Protocol compliance verification

## Submitting Changes

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run all quality checks
5. Add tests for new functionality
6. Update documentation as needed
7. Submit a pull request

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write clear, self-documenting code
- Add comments for public APIs
- Keep generated files separate from hand-written code

## Schema Updates

When the upstream ACP schema changes:

1. Run `make generate` to update generated types
2. Fix any breaking changes in hand-written code
3. Update examples if needed
4. Test compatibility with reference implementations
5. Update version information if protocol version changed

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Provide minimal reproducible examples for bugs
- Include relevant version information