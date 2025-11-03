# ClusterKit CLI Makefile

# Version information
VERSION ?= dev
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -X 'github.com/clusterkit/clusterkit/cmd/clusterkit/cmd.Version=$(VERSION)' \
           -X 'github.com/clusterkit/clusterkit/cmd/clusterkit/cmd.GitCommit=$(GIT_COMMIT)' \
           -X 'github.com/clusterkit/clusterkit/cmd/clusterkit/cmd.BuildDate=$(BUILD_DATE)'

# Binary name
BINARY := clusterkit
MAIN := ./cmd/clusterkit

.PHONY: all build install clean test fmt lint help

all: build

## build: Build the binary
build:
	@echo "Building $(BINARY) $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN)
	@echo "✓ Built: $(BINARY)"

## install: Install the binary to /usr/local/bin
install: build
	@echo "Installing $(BINARY) to /usr/local/bin..."
	@sudo mv $(BINARY) /usr/local/bin/
	@echo "✓ Installed: /usr/local/bin/$(BINARY)"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY)
	@rm -rf dist/
	@echo "✓ Cleaned"

## test: Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Formatted"

## lint: Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@golangci-lint run || echo "Note: golangci-lint not installed"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✓ Dependencies downloaded"

## tidy: Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "✓ Dependencies tidied"

## release: Build release binaries for multiple platforms
release:
	@echo "Building release binaries..."
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 $(MAIN)
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 $(MAIN)
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 $(MAIN)
	@echo "✓ Release binaries built in dist/"

## run: Run the CLI locally
run: build
	@./$(BINARY)

## help: Show this help
help:
	@echo "ClusterKit CLI Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
