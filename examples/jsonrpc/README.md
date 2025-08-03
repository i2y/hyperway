# JSON-RPC 2.0 Example

This example demonstrates how to use Hyperway's JSON-RPC 2.0 support alongside gRPC, Connect, and gRPC-Web protocols.

## Features Demonstrated

- Single JSON-RPC requests
- Batch JSON-RPC requests
- Error handling
- Input validation
- Simultaneous protocol support (gRPC, Connect, gRPC-Web, JSON-RPC)

## Running the Example

```bash
go run main.go
```

Then open http://localhost:8080/index.html in your browser to try the interactive demo.

## JSON-RPC Endpoints

The JSON-RPC endpoint is available at: `http://localhost:8080/api/jsonrpc`

### Example Requests

#### Single Request
```bash
curl -X POST http://localhost:8080/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "add",
    "params": {"a": 5, "b": 3},
    "id": 1
  }'
```

#### Batch Request
```bash
curl -X POST http://localhost:8080/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '[
    {"jsonrpc": "2.0", "method": "add", "params": {"a": 1, "b": 2}, "id": 1},
    {"jsonrpc": "2.0", "method": "multiply", "params": {"a": 3, "b": 4}, "id": 2}
  ]'
```

#### Notification (no response expected)
```bash
curl -X POST http://localhost:8080/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "add",
    "params": {"a": 5, "b": 3}
  }'
```

## Other Protocol Support

The same service also supports:

### gRPC
```bash
grpcurl -plaintext -d '{"a": 5, "b": 3}' \
  localhost:8080 calculator.v1.CalculatorService/add
```

### Connect
```bash
curl -X POST http://localhost:8080/calculator.v1.CalculatorService/add \
  -H "Content-Type: application/json" \
  -d '{"a": 5, "b": 3}'
```

### gRPC-Web
```bash
curl -X POST http://localhost:8080/calculator.v1.CalculatorService/add \
  -H "Content-Type: application/grpc-web+json" \
  -d '{"a": 5, "b": 3}'
```
