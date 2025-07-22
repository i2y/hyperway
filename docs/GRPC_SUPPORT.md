# gRPC Protocol Support in Hyperway

Hyperway provides full gRPC protocol support, including compatibility with standard gRPC tools like `grpcurl`.

## Features

- **Full gRPC Protocol Support**: Complete implementation of the gRPC protocol over HTTP/2
- **gRPC Reflection**: Built-in reflection support for service discovery
- **grpcurl Compatibility**: Works seamlessly with grpcurl and other gRPC tools
- **Multi-Protocol**: Simultaneously supports gRPC and Connect RPC protocols

## Setting Up gRPC Support

### 1. Enable Reflection

```go
svc := rpc.NewService("MyService",
    rpc.WithPackage("myapp.v1"),
    rpc.WithReflection(true), // Enable gRPC reflection
)
```

### 2. Use HTTP/2 with h2c

```go
import (
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

gateway, err := rpc.NewGateway(svc)
if err != nil {
    log.Fatal(err)
}

srv := &http.Server{
    Addr: ":8080",
    Handler: h2c.NewHandler(gateway, &http2.Server{}),
}
```

## Testing with grpcurl

### List Available Services

```bash
grpcurl -plaintext localhost:8080 list
```

Output:
```
grpc.example.v1.UserService
grpc.reflection.v1.ServerReflection
grpc.reflection.v1alpha.ServerReflection
```

### Describe a Service

```bash
grpcurl -plaintext localhost:8080 describe grpc.example.v1.UserService
```

Output:
```
grpc.example.v1.UserService is a service:
service UserService {
  rpc CreateUser ( .grpc.example.v1.CreateUserRequest ) returns ( .grpc.example.v1.CreateUserResponse );
  rpc DeleteUser ( .grpc.example.v1.DeleteUserRequest ) returns ( .grpc.example.v1.DeleteUserResponse );
  rpc GetUser ( .grpc.example.v1.GetUserRequest ) returns ( .grpc.example.v1.GetUserResponse );
  rpc ListUsers ( .grpc.example.v1.ListUsersRequest ) returns ( .grpc.example.v1.ListUsersResponse );
}
```

### Make RPC Calls

```bash
# Create a user
grpcurl -plaintext -d '{"name":"Alice","email":"alice@example.com"}' \
    localhost:8080 grpc.example.v1.UserService/CreateUser

# Get a user
grpcurl -plaintext -d '{"id":"user-1"}' \
    localhost:8080 grpc.example.v1.UserService/GetUser

# List users
grpcurl -plaintext -d '{"limit":10,"offset":0}' \
    localhost:8080 grpc.example.v1.UserService/ListUsers
```

## Protocol Detection

Hyperway automatically detects the protocol based on request headers:

- **gRPC**: Content-Type: `application/grpc` or `application/grpc+proto`
- **Connect RPC**: Connect-Protocol-Version header present or Content-Type: `application/json` without gRPC headers

## Advanced Features

### Custom Metadata

Send custom headers with grpcurl:

```bash
grpcurl -plaintext \
    -H "Authorization: Bearer token123" \
    -H "X-Request-ID: req-456" \
    -d '{"id":"user-1"}' \
    localhost:8080 grpc.example.v1.UserService/GetUser
```

### Error Handling

gRPC errors are properly mapped to status codes:

```go
import "google.golang.org/grpc/codes"
import "google.golang.org/grpc/status"

func handler(ctx context.Context, req *Request) (*Response, error) {
    // Return gRPC-style errors
    return nil, status.Error(codes.NotFound, "user not found")
}
```

### Streaming Support

Note: Hyperway currently supports unary RPCs only. Streaming support is planned for future releases.

## Debugging

### Enable Debug Logging

```bash
grpcurl -plaintext -v \
    -d '{"id":"user-1"}' \
    localhost:8080 grpc.example.v1.UserService/GetUser
```

### List Methods with Details

```bash
grpcurl -plaintext localhost:8080 list grpc.example.v1.UserService
```

### Describe Message Types

```bash
grpcurl -plaintext localhost:8080 describe .grpc.example.v1.CreateUserRequest
```

## Performance Considerations

- Use HTTP/2 for better connection multiplexing
- Enable connection pooling in clients
- Consider using binary protobuf encoding for better performance
- Profile with PGO for optimized message parsing

## Limitations

- No streaming RPC support (unary only)
- No client-side load balancing
- No built-in retry policies
- No compression support yet

## Example Application

See the complete example in `examples/grpc/main.go` for a full implementation with:
- Multiple service methods
- Validation
- Error handling
- gRPC reflection
- grpcurl compatibility
