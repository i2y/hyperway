# Hyperway API Documentation

## Table of Contents
- [Service Creation](#service-creation)
- [Method Registration](#method-registration)
- [Gateway Configuration](#gateway-configuration)
- [Type Mapping](#type-mapping)
- [Validation](#validation)
- [Error Handling](#error-handling)

## Service Creation

### `rpc.NewService(name string, opts ...ServiceOption) *Service`

Creates a new RPC service.

```go
svc := rpc.NewService("UserService", 
    rpc.WithPackage("user.v1"),      // Set protobuf package
    rpc.WithValidation(true),        // Enable validation
    rpc.WithReflection(true),        // Enable gRPC reflection
)
```

### Service Options

- `rpc.WithPackage(pkg string)` - Sets the protobuf package name
- `rpc.WithValidation(enabled bool)` - Enables/disables validation for all methods
- `rpc.WithReflection(enabled bool)` - Enables/disables gRPC reflection

## Method Registration

### `rpc.NewMethod(name string, handler interface{}) *MethodBuilder`

Creates a new method builder for fluent configuration.

Handler signature must be:
```go
func(context.Context, *RequestType) (*ResponseType, error)
```

### Method Builder API

```go
method := rpc.NewMethod("CreateUser", handler).
    In(RequestType{}).       // Set request type
    Out(ResponseType{}).     // Set response type
    Validate(true).          // Override validation setting
    Description("...")       // Add method description
```

### Registering Methods

```go
// Single method
err := svc.Register(method.Build())

// Multiple methods with MustRegister
rpc.MustRegister(svc,
    rpc.NewMethod("Create", createHandler).In(CreateReq{}).Out(CreateResp{}),
    rpc.NewMethod("Get", getHandler).In(GetReq{}).Out(GetResp{}),
    rpc.NewMethod("Update", updateHandler).In(UpdateReq{}).Out(UpdateResp{}),
)
```

## Gateway Configuration

### `rpc.NewGateway(services ...*Service) (http.Handler, error)`

Creates a gateway that supports multiple protocols:
- gRPC (Protobuf only)
- Connect RPC (both Protobuf and JSON formats)

```go
gateway, err := rpc.NewGateway(userSvc, adminSvc)
if err != nil {
    log.Fatal(err)
}

// Start server with HTTP/2 support
srv := &http.Server{
    Addr:    ":8080",
    Handler: h2c.NewHandler(gateway, &http2.Server{}),
}
```

## Type Mapping

Go types are mapped to Protobuf types as follows:

| Go Type | Protobuf Type | Notes |
|---------|---------------|-------|
| `string` | `string` | |
| `bool` | `bool` | |
| `int32` | `int32` | |
| `int`, `int64` | `int64` | |
| `uint32` | `uint32` | |
| `uint`, `uint64` | `uint64` | |
| `float32` | `float` | |
| `float64` | `double` | |
| `[]byte` | `bytes` | |
| `[]T` | `repeated T` | Slices become repeated fields |
| `map[K]V` | `map<K,V>` | Map keys must be strings or integers |
| `struct` | `message` | Nested structs become nested messages |
| `*T` | `T` | Pointers indicate optional fields |

### Struct Tags

Use JSON tags to control field names:
```go
type User struct {
    ID        string `json:"id"`
    FirstName string `json:"first_name"`
    IsActive  bool   `json:"is_active"`
    Internal  string `json:"-"` // Excluded from proto
}
```

## Validation

Hyperway integrates with [go-playground/validator](https://github.com/go-playground/validator).

### Common Validation Tags

```go
type Request struct {
    // Required fields
    Name string `json:"name" validate:"required"`
    
    // String validation
    Email    string `json:"email" validate:"required,email"`
    Username string `json:"username" validate:"required,alphanum,min=3,max=20"`
    
    // Numeric validation
    Age   int     `json:"age" validate:"min=0,max=150"`
    Price float64 `json:"price" validate:"gt=0"`
    
    // Slice validation
    Tags []string `json:"tags" validate:"required,min=1,max=10,dive,min=1,max=20"`
    
    // Nested validation
    Address Address `json:"address" validate:"required,dive"`
}
```

### Custom Validation

```go
// Register custom validator
svc.Validator().RegisterValidation("customtag", func(fl validator.FieldLevel) bool {
    // Custom validation logic
    return true
})
```

## Error Handling

Errors returned from handlers are automatically converted to appropriate protocol errors:

```go
func handler(ctx context.Context, req *Request) (*Response, error) {
    // Validation errors (automatic with validation enabled)
    
    // Business logic errors
    if !isAuthorized {
        return nil, errors.New("unauthorized")
    }
    
    // Not found
    if !exists {
        return nil, errors.New("resource not found")
    }
    
    // Generic errors
    return nil, fmt.Errorf("failed to process: %w", err)
}
```

### Protocol-Specific Error Handling

- **gRPC**: Errors are mapped to gRPC status codes
- **Connect RPC**: Errors are returned in Connect error format with appropriate HTTP status codes

## Advanced Usage

### Interceptors (Coming Soon)

```go
// Global interceptor
svc := rpc.NewService("Service",
    rpc.WithInterceptors(loggingInterceptor, authInterceptor),
)

// Method-specific interceptor
method := rpc.NewMethod("SecureMethod", handler).
    Interceptors(authInterceptor)
```

### Context Values

Access service metadata in handlers:

```go
func handler(ctx context.Context, req *Request) (*Response, error) {
    // Access HTTP headers (if available)
    if md, ok := metadata.FromIncomingContext(ctx); ok {
        auth := md.Get("authorization")
    }
    
    // Standard context values work as expected
    userID := ctx.Value("userID")
    
    return &Response{}, nil
}
```

## Performance Tips

1. **Reuse Services**: Create services once and reuse them
2. **Enable Pooling**: Message pooling is enabled by default in codecs
3. **Use HTTP/2**: Better performance for gRPC and multiplexing
4. **Batch Operations**: Design APIs to support batch operations when possible

## Debugging

### Enable Debug Logging

```go
import "log/slog"

slog.SetLogLoggerLevel(slog.LevelDebug)
```

### Test with Different Clients

```bash
# Connect RPC (JSON format)
curl -X POST http://localhost:8080/service/method \
  -H "Content-Type: application/json" \
  -d '{...}'

# Connect Protocol  
curl -X POST http://localhost:8080/service/method \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{...}'

# gRPC (requires HTTP/2)
grpcurl -plaintext -d '{...}' localhost:8080 service/method
```

### View OpenAPI Spec

```bash
curl http://localhost:8080/openapi.json | jq
```