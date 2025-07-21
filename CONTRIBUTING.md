# Contributing to Hyperway

Thank you for your interest in contributing to Hyperway! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to be respectful and constructive in all interactions.

## How to Contribute

### Reporting Issues

- Check if the issue already exists
- Provide a clear description of the problem
- Include steps to reproduce
- Share relevant code snippets or error messages
- Mention your Go version and OS

### Submitting Pull Requests

1. **Fork the repository**
   ```bash
   git clone https://github.com/i2y/hyperway.git
   cd hyperway
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
   - Follow the existing code style
   - Add tests for new functionality
   - Update documentation as needed

4. **Run tests and linting**
   ```bash
   make test
   make lint
   ```

5. **Commit your changes**
   - Use clear, descriptive commit messages
   - Reference any related issues

6. **Push and create a Pull Request**
   - Provide a clear description of your changes
   - Link to any related issues
   - Ensure all CI checks pass

## Development Setup

### Prerequisites

- Go 1.22 or higher
- Make (for running Makefile commands)

### Building

```bash
# Build all packages
make build

# Build CLI
make build-cli

# Install CLI
make install-cli
```

### Testing

```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run tests with coverage
make test-cover

# Run benchmarks
make bench
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `make fmt` before committing
- Ensure `make lint` passes

## Project Structure

```
hyperway/
├── cmd/          # CLI tool
├── codec/        # Hyperpb codec implementation
├── examples/     # Example applications
├── gateway/      # HTTP gateway with multi-protocol support
├── internal/     # Internal packages
├── proto/        # Proto export functionality
├── router/       # Request routing logic
├── rpc/          # Main RPC API
├── schema/       # Dynamic schema generation
└── test/         # Integration tests
```

## Adding New Features

1. Discuss major changes in an issue first
2. Write tests for new functionality
3. Update relevant documentation
4. Add examples if applicable

## Documentation

- Update README.md for user-facing changes
- Add godoc comments for all exported types and functions
- Update SUPPORTED_FEATURES.md for new capabilities

## Release Process

Releases are managed by maintainers. To request a release:

1. Ensure all tests pass
2. Update CHANGELOG.md (if present)
3. Create an issue requesting a release

## Questions?

Feel free to open an issue for any questions about contributing!
