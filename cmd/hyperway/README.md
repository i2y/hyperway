# Hyperway CLI

The hyperway CLI provides tools for managing proto files and services in the hyperway RPC framework.

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/i2y/hyperway.git
cd hyperway

# Install the CLI
make install-cli

# Or build to ./build directory
make build-cli
```

### Using Go Install

```bash
go install github.com/i2y/hyperway/cmd/hyperway@latest
```

## Usage

### Proto Export

Export proto files from a running hyperway service:

```bash
# Export to current directory
hyperway proto export --endpoint http://localhost:8080

# Export to specific directory
hyperway proto export --endpoint http://localhost:8080 --output ./protos

# Export as ZIP archive
hyperway proto export --endpoint http://localhost:8080 --format zip --output service.zip

# Export without comments and sorted
hyperway proto export --endpoint http://localhost:8080 --no-comments --sort
```

### Proto Generate (Planned)

Generate proto files from Go source code:

```bash
# Generate from current directory
hyperway proto generate

# Generate from specific directory
hyperway proto generate --input ./model

# Generate for specific packages
hyperway proto generate --packages model,api

# Generate recursively
hyperway proto generate --recursive --output ./protos
```

### Serve (Planned)

Start a hyperway RPC server:

```bash
# Start server on default port
hyperway serve

# Start server on specific port
hyperway serve --port 9090

# Start with configuration file
hyperway serve --config server.yaml

# Enable all features
hyperway serve --reflection --openapi --metrics
```

### Version

Show version information:

```bash
hyperway version
```

## Commands

### `hyperway proto export`

Export proto files from a running service using gRPC reflection.

**Flags:**
- `-e, --endpoint string`: Service endpoint URL (default "http://localhost:8080")
- `-o, --output string`: Output directory or file for ZIP (default ".")
- `-f, --format string`: Output format: files or zip (default "files")
- `--comments`: Include comments in proto files (default true)
- `--sort`: Sort proto elements alphabetically
- `--timeout duration`: Request timeout (default 30s)

### `hyperway proto generate`

Generate proto files from Go source code (not yet implemented).

**Flags:**
- `-i, --input string`: Input directory containing Go source files (default ".")
- `-o, --output string`: Output directory for generated proto files (default "./proto")
- `-p, --packages strings`: Specific packages to generate (comma-separated)
- `--comments`: Include Go comments in proto files (default true)
- `--sort`: Sort proto elements alphabetically
- `-r, --recursive`: Process directories recursively

### `hyperway serve`

Start a hyperway RPC server (not yet implemented).

**Flags:**
- `-p, --port int`: Server port (default 8080)
- `--host string`: Server host (default "0.0.0.0")
- `-c, --config string`: Configuration file path
- `--reflection`: Enable gRPC reflection (default true)
- `--openapi`: Enable OpenAPI endpoint (default true)
- `--metrics`: Enable metrics endpoint
- `--graceful-timeout duration`: Graceful shutdown timeout (default 30s)

### `hyperway version`

Display version information including:
- Version number
- Git commit hash
- Build date
- Go version
- OS/Architecture

## Examples

### Export Proto Files from a Service

```bash
# Start your hyperway service
cd examples/grpc
go run main.go

# In another terminal, export the proto files
hyperway proto export --endpoint http://localhost:8080 --output ./exported-protos
```

### Build with Version Information

```bash
# Build with version info
make build-cli

# Check version
./build/hyperway version
```

## Future Features

- **Proto Generation**: Analyze Go source code and generate corresponding proto files
- **Service Management**: Start and manage hyperway services
- **Schema Validation**: Validate proto schemas and Go struct compatibility
- **Migration Tools**: Convert between different RPC frameworks
- **Performance Testing**: Built-in load testing and benchmarking tools