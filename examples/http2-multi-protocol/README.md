# Multi-Protocol HTTP/2 Example

This example demonstrates Hyperway's ability to support multiple RPC protocols simultaneously on a single server with HTTP/1.1 and HTTP/2 support.

## Supported Protocols

- **gRPC** - Requires HTTP/2
- **Connect-RPC** - Works with both HTTP/1.1 and HTTP/2
- **gRPC-Web** - Works with both HTTP/1.1 and HTTP/2
- **JSON-RPC 2.0** - Works with both HTTP/1.1 and HTTP/2

## Running the Example

1. Start the server:
```bash
go run main.go
```

2. Run the test script:
```bash
./test_protocols.sh
```

## Manual Testing

### JSON-RPC
```bash
# HTTP/1.1
curl -X POST http://localhost:8084/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "add", "params": {"a": 5, "b": 3}, "id": 1}'

# HTTP/2
curl --http2-prior-knowledge -X POST http://localhost:8084/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "add", "params": {"a": 5, "b": 3}, "id": 1}'
```

### gRPC (using buf curl)
```bash
buf curl --protocol grpc \
  --schema calc.proto \
  --http2-prior-knowledge \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add
```

### Connect-RPC
```bash
buf curl --protocol connect \
  --schema calc.proto \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add
```

### gRPC-Web
```bash
buf curl --protocol grpcweb \
  --schema calc.proto \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add
```

## Technical Details

The server uses:
- `h2c` (HTTP/2 cleartext) to support both HTTP/1.1 and HTTP/2 on the same port
- Hyperway's multi-protocol gateway to handle different RPC protocols
- Protocol detection based on HTTP headers and URL paths

### Protocol Detection

- **JSON-RPC**: Detected by URL path ending with `/jsonrpc` or Content-Type `application/json-rpc`
- **gRPC**: Detected by Content-Type `application/grpc`
- **Connect**: Detected by `Connect-Protocol-Version: 1` header
- **gRPC-Web**: Detected by `X-Grpc-Web: 1` header or Content-Type containing `grpc-web`
