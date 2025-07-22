# Protocol Support in Hyperway

This document clarifies the exact protocol support in Hyperway.

## Supported Protocols

Hyperway supports two RPC protocols:

### 1. gRPC Protocol
- **Wire Format**: Protobuf binary only
- **Transport**: HTTP/2 required
- **Content-Type**: `application/grpc` or `application/grpc+proto`
- **JSON Support**: ❌ Not supported (gRPC-JSON transcoding is not implemented)
- **Testing Tool**: grpcurl

### 2. Connect RPC Protocol
- **Wire Formats**: 
  - Protobuf binary (`application/proto`)
  - JSON (`application/json`)
- **Transport**: Works over both HTTP/1.1 and HTTP/2
- **Protocol Detection**: 
  - With `Connect-Protocol-Version: 1` header
  - Without header: JSON requests are treated as Connect RPC
- **Testing Tool**: curl, buf curl, or any HTTP client

## What About "REST API" Support?

**Important**: Hyperway does NOT provide generic REST API support. When documentation mentions "REST", it specifically refers to Connect RPC's JSON format, which:

- Uses POST requests to RPC endpoints (e.g., `/service.v1.UserService/CreateUser`)
- Accepts and returns JSON payloads
- Is NOT a RESTful API with resources and HTTP verbs
- Is fully compatible with the Connect RPC specification

## OpenAPI Generation

Hyperway can generate OpenAPI specifications for documentation purposes. This allows:
- API documentation generation
- Use with tools like Swagger UI
- Client SDK generation

However, the actual API endpoints follow RPC conventions, not REST conventions.

## Protocol Comparison

| Feature | gRPC | Connect RPC |
|---------|------|-------------|
| Binary Format | ✅ Protobuf | ✅ Protobuf |
| JSON Format | ❌ | ✅ |
| HTTP/1.1 | ❌ | ✅ |
| HTTP/2 | ✅ | ✅ |
| Streaming | ❌ (planned) | ❌ (planned) |
| Web Browser Support | ❌ | ✅ |
| grpcurl Support | ✅ | ❌ |
| curl Support | ❌ | ✅ |

## Implementation Details

### Why No gRPC-JSON Transcoding?

The current implementation does not use Vanguard-go or similar transcoding libraries, which means:

1. gRPC requests must use Protobuf binary format
2. JSON requests are handled by Connect RPC protocol
3. There is no automatic conversion between gRPC and JSON

### Future Considerations

To add true gRPC-JSON transcoding support, the implementation would need to:
- Integrate Vanguard-go or similar transcoding library
- Implement the gRPC-JSON transcoding specification
- Handle the complexity of JSON-to-Protobuf field mapping

## Testing Examples

### gRPC (Protobuf only)
```bash
# Uses binary Protobuf format
grpcurl -plaintext -d '{"name":"Alice"}' \
    localhost:8080 user.v1.UserService/CreateUser
```

### Connect RPC (JSON)
```bash
# Uses JSON format
curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
    -H "Content-Type: application/json" \
    -d '{"name":"Alice"}'
```

### Connect RPC (Protobuf)
```bash
# Uses binary Protobuf format with Connect protocol
buf curl --protocol connect \
    --data '{"name":"Alice"}' \
    http://localhost:8080/user.v1.UserService/CreateUser
```

## Summary

- **gRPC**: Protobuf-only, requires HTTP/2
- **Connect RPC**: Supports both JSON and Protobuf, works with HTTP/1.1 and HTTP/2
- **No REST API**: The JSON support is specifically Connect RPC's JSON format, not a RESTful API
- **No gRPC-JSON**: gRPC protocol does not support JSON in the current implementation
