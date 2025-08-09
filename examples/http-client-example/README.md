# HTTP Client Example with Hyperway Server

This example demonstrates how to communicate with a Hyperway server using a simple HTTP client. It shows that Hyperway servers are accessible via standard HTTP/JSON, making them easy to integrate with any HTTP client library or tool.

## 📋 Purpose

This example is useful when:
- You want to quickly test a Hyperway server without code generation
- You're using a language that doesn't have Connect/gRPC support
- You need a lightweight client implementation
- You want to understand the Connect protocol at the HTTP level

**Note**: For production Go applications, we recommend using the proper Connect-go client with code generation. See the [connect-go-client example](../connect-go-client).

## Overview

This example includes:
- **Hyperway Server**: A user management service with CRUD operations
- **HTTP Client**: A simple Go HTTP client using only standard library
- **Protocol Details**: Raw Connect protocol communication over HTTP/JSON

## Features Demonstrated

1. **HTTP/JSON Communication**
   - Direct HTTP POST requests to RPC endpoints
   - JSON request/response encoding
   - No code generation required

2. **Connect Protocol**
   - Connect protocol headers
   - Error handling with Connect error format
   - Works with any HTTP client

3. **Server Features**
   - Validation with struct tags
   - In-memory data storage
   - Multi-protocol support (also works with gRPC/gRPC-Web)

## Running the Example

### Start the Server

```bash
go run server.go
```

The server starts on `http://localhost:8080` and supports:
- Connect protocol (JSON and Protobuf)
- gRPC protocol
- gRPC-Web protocol
- All accessible via HTTP/2 and HTTP/1.1

### Run the HTTP Client

```bash
go run client.go
```

The client performs:
1. Create a user
2. Get the user
3. Create another user
4. List all users
5. Update a user
6. Delete a user
7. Verify deletion

## Code Structure

### Server (`server.go`)
- Defines data models using Go structs
- Implements service handlers
- No `.proto` files needed
- Automatic schema generation at runtime

### Client (`client.go`)
- Uses standard `net/http` package
- Demonstrates Connect protocol over HTTP
- JSON marshaling/unmarshaling
- Error handling

## Testing with cURL

Since this is just HTTP/JSON, you can test with cURL:

```bash
# Create a user
curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name":"Alice","email":"alice@example.com"}'

# Get a user
curl -X POST http://localhost:8080/user.v1.UserService/GetUser \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"id":"user-123"}'

# List users
curl -X POST http://localhost:8080/user.v1.UserService/ListUsers \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"page_size":10}'
```

## Connect Protocol Details

The Connect protocol uses HTTP POST requests with:
- Path: `/{package}.{service}/{method}`
- Content-Type: `application/json` (or `application/proto` for Protobuf)
- Optional: `Connect-Protocol-Version: 1` header

Errors are returned as JSON with:
```json
{
  "code": "not_found",
  "message": "user not found"
}
```

## When to Use This Approach

✅ **Good for:**
- Quick prototyping and testing
- Simple integrations
- Languages without Connect/gRPC support
- Understanding the protocol

❌ **Not recommended for:**
- Production Go applications (use proper Connect-go client)
- When you need type safety
- When you need advanced features (streaming, interceptors, etc.)

## Comparison with Connect-go Client

| Feature | HTTP Client (this) | Connect-go Client |
|---------|-------------------|-------------------|
| Code Generation | ❌ Not needed | ✅ Required |
| Type Safety | ❌ Runtime only | ✅ Compile-time |
| Setup Complexity | ✅ Simple | 📝 Requires proto export |
| Interceptors | ❌ Manual | ✅ Built-in |
| Streaming | ❌ Not supported | ✅ Supported |
| Production Ready | ⚠️ Basic | ✅ Full-featured |

## Next Steps

For production use, consider:
1. [Connect-go client example](../connect-go-client) - Full Connect-go integration
2. Export proto files for cross-language support
3. Add proper error handling and retries
4. Implement authentication and interceptors

## Key Takeaway

Hyperway servers are just HTTP servers! You can access them with any HTTP client, making integration simple and universal. While this example uses Go's `net/http`, the same approach works with:
- Python's `requests`
- JavaScript's `fetch`
- Java's `HttpClient`
- Any HTTP client in any language