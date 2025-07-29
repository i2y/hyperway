# Protocol Support in Hyperway

This document clarifies the exact protocol support in Hyperway.

## Supported Protocols

Hyperway supports three RPC protocols:

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

### 3. gRPC-Web Protocol
- **Wire Formats**:
  - Protobuf binary (`application/grpc-web` or `application/grpc-web+proto`)
  - Base64-encoded (`application/grpc-web-text` or `application/grpc-web-text+proto`)
- **Transport**: HTTP/1.1 (works with HTTP/2 as well)
- **Content-Type**: `application/grpc-web*` variants
- **Message Framing**: 5-byte header (1 flag + 4 length) + payload
- **Trailer Handling**: Trailers sent as final message with flag 0x80
- **Browser Support**: ✅ Full support without proxy
- **Testing Tool**: Browser clients, gRPC-Web libraries

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

| Feature | gRPC | Connect RPC | gRPC-Web |
|---------|------|-------------|----------|
| Binary Format | ✅ Protobuf | ✅ Protobuf | ✅ Protobuf |
| JSON Format | ❌ | ✅ | ❌ |
| Base64 Encoding | ❌ | ❌ | ✅ |
| HTTP/1.1 | ❌ | ✅ | ✅ |
| HTTP/2 | ✅ | ✅ | ✅ |
| Streaming | ❌ (planned) | ❌ (planned) | ❌ (unary only) |
| Web Browser Support | ❌ | ✅ | ✅ |
| CORS Support | ❌ | ✅ | ✅ |
| grpcurl Support | ✅ | ❌ | ❌ |
| curl Support | ❌ | ✅ | ❌ |
| Requires Proxy | N/A | ❌ | ❌ |

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

### gRPC-Web (Browser)
```javascript
// Using gRPC-Web client library
const client = new UserServiceClient('http://localhost:8080');
const request = new CreateUserRequest();
request.setName('Alice');

client.createUser(request, {}, (err, response) => {
  if (err) {
    console.error('Error:', err);
  } else {
    console.log('Response:', response.toObject());
  }
});
```

### gRPC-Web (Manual Testing)
```bash
# Create a gRPC-Web frame manually (for testing)
# Note: This is a simplified example
echo -n '{"name":"Alice"}' | \
  awk '{printf "\x00\x00\x00\x00%c%s", length($0), $0}' | \
  base64 | \
  curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
    -H "Content-Type: application/grpc-web-text" \
    -H "X-Grpc-Web: 1" \
    --data-binary @-
```

## Summary

- **gRPC**: Protobuf-only, requires HTTP/2
- **Connect RPC**: Supports both JSON and Protobuf, works with HTTP/1.1 and HTTP/2
- **gRPC-Web**: Browser-friendly protocol with base64 encoding support, works with HTTP/1.1
- **No REST API**: The JSON support is specifically Connect RPC's JSON format, not a RESTful API
- **No gRPC-JSON**: gRPC protocol does not support JSON in the current implementation
- **Multi-Protocol**: All three protocols are served on the same port with automatic detection
