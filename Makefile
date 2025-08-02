.PHONY: all build test lint fmt clean install-tools help

# Variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOVET := $(GOCMD) vet
GOLINT := $(HOME)/go/bin/golangci-lint
GOCOVER := $(GOCMD) tool cover

# Build variables
BINARY_NAME := hyperway
BUILD_DIR := ./build
COVERAGE_FILE := coverage.out
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

# Package lists
PACKAGES := $(shell $(GOCMD) list ./... | grep -v /vendor/)
SOURCE_FILES := $(shell find . -name '*.go' -type f -not -path "./vendor/*")

# Default target
all: fmt lint test build

# Help target
help:
	@echo "Available targets:"
	@echo "  make all          - Format, lint, test, and build"
	@echo "  make build        - Build the project"
	@echo "  make build-cli    - Build the hyperway CLI binary"
	@echo "  make install-cli  - Install the hyperway CLI to GOPATH/bin"
	@echo "  make test         - Run tests"
	@echo "  make test-v       - Run tests with verbose output"
	@echo "  make test-race    - Run tests with race detector"
	@echo "  make test-cover   - Run tests with coverage"
	@echo "  make bench        - Run benchmarks"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run go vet"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make deps         - Download dependencies"
	@echo "  make deps-update  - Update dependencies"
	@echo "  make install-tools - Install required tools"

# Build the project
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -v ./...

# Build the CLI binary
build-cli:
	@echo "Building hyperway CLI..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/hyperway

# Install the CLI binary to $GOPATH/bin
install-cli:
	@echo "Installing hyperway CLI..."
	@$(GOCMD) install -ldflags "$(LDFLAGS)" ./cmd/hyperway

# Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

# Run tests with verbose output
test-v:
	@echo "Running tests (verbose)..."
	@$(GOTEST) -v -count=1 ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@$(GOTEST) -race -v ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@$(GOCOVER) -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./...

# Run linter
lint:
	@echo "Running linter..."
	@if [ ! -f $(GOLINT) ]; then \
		echo "golangci-lint not found. Installing..."; \
		make install-tools; \
	fi
	@$(GOLINT) run

# Run linter with auto-fix
lint-fix:
	@echo "Running linter with auto-fix..."
	@if [ ! -f $(GOLINT) ]; then \
		echo "golangci-lint not found. Installing..."; \
		make install-tools; \
	fi
	@$(GOLINT) run --fix

# Format code
fmt:
	@echo "Formatting code..."
	@$(GOFMT) -w $(SOURCE_FILES)
	@$(GOCMD) fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@$(GOVET) ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE) coverage.html
	@$(GOCMD) clean -cache

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	@$(GOCMD) get -u ./...
	@$(GOMOD) tidy

# Install required tools
install-tools:
	@echo "Installing tools..."
	@$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2
	@echo "Tools installed successfully"

# Check if all tools are installed
check-tools:
	@echo "Checking tools..."
	@which $(GOLINT) > /dev/null || (echo "golangci-lint not found. Run 'make install-tools'" && exit 1)
	@echo "All tools are installed"

# Run CI pipeline (used in CI/CD)
ci: deps lint test-race

# Development workflow - format, lint, and test
dev: fmt lint test

# Quick check before commit
pre-commit: fmt lint test

# Generate mocks (placeholder for future use)
generate:
	@echo "Running go generate..."
	@$(GOCMD) generate ./...

# Show test coverage in terminal
cover-report:
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./... > /dev/null 2>&1
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE)

# Show which packages have low test coverage
cover-check:
	@echo "Checking test coverage..."
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./... > /dev/null 2>&1
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE) | grep -E "^total:" | awk '{print "Total coverage: " $$3}'
