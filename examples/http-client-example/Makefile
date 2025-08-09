.PHONY: all server client test bench clean help

help:
	@echo "Available targets:"
	@echo "  make server    - Run the Hyperway server"
	@echo "  make client    - Run the Connect-go client"
	@echo "  make test      - Run integration tests"
	@echo "  make bench     - Run benchmarks"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make all       - Run tests and benchmarks"

server:
	@echo "Starting Hyperway server on :8080..."
	go run server.go

client:
	@echo "Running Connect-go client..."
	go run client.go

test:
	@echo "Running integration tests..."
	go test -v -race

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem

test-cover:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

all: test bench

clean:
	@echo "Cleaning build artifacts..."
	rm -f coverage.out coverage.html
	go clean

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy