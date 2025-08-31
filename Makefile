.PHONY: setup fetch-schema generate build test lint clean examples

# Install required tools and dependencies
setup:
	go install github.com/atombender/go-jsonschema@latest
	go mod download

# Download latest schema from upstream ACP repository
fetch-schema:
	@mkdir -p schema
	@echo "Fetching latest schema from upstream..."
	@curl -s -o schema/schema.json https://raw.githubusercontent.com/zed-industries/agent-client-protocol/main/schema/schema.json
	@curl -s -o schema/meta.json https://raw.githubusercontent.com/zed-industries/agent-client-protocol/main/schema/meta.json
	@echo "Schema files downloaded successfully"

# Generate Go types from schema
generate: fetch-schema
	@echo "Generating Go types from schema..."
	@$(shell go env GOPATH)/bin/go-jsonschema -p acp --only-models --tags json,yaml schema/schema.json > acp/types_generated.go
	@echo "Generating constants from meta.json..."
	@go run cmd/generate/main.go constants
	@go fmt ./...
	@echo "Code generation completed"

# Build all packages
build:
	go build ./...

# Run tests
test:
	go test ./...

# Run linters
lint:
	golangci-lint run

fmt:
	go fmt ./...

vet:
	go vet ./...

check: test fmt vet lint

# Build examples
examples: build
	go build -o bin/example-agent examples/agent/
	go build -o bin/example-client examples/client/
