# Hyperway API Documentation

## Table of Contents
- [Service Creation](#service-creation)
- [Method Registration](#method-registration)
- [Gateway Configuration](#gateway-configuration)
- [Type Mapping](#type-mapping)
- [Validation](#validation)
- [Error Handling](#error-handling)
- [Interceptors](#interceptors)
- [Proto Export](#proto-export)

## Service Creation

### `rpc.NewService(name string, opts ...ServiceOption) *Service`

Creates a new RPC service.

```go
svc := rpc.NewService("UserService", 
    rpc.WithPackage("user.v1"),              // Set protobuf package
    rpc.WithValidation(true),                // Enable validation
    rpc.WithReflection(true),                // Enable gRPC reflection
    rpc.WithInterceptors(loggingInterceptor), // Add interceptors
    rpc.WithDescription("User management service"), // Add service description
)
```

### Service Options

- `rpc.WithPackage(pkg string)` - Sets the protobuf package name
- `rpc.WithValidation(enabled bool)` - Enables/disables validation for all methods
- `rpc.WithReflection(enabled bool)` - Enables/disables gRPC reflection
- `rpc.WithInterceptors(interceptors ...Interceptor)` - Adds interceptors to all methods
- `rpc.WithEdition(edition string)` - Sets Protobuf Edition (e.g., "2023")
- `rpc.WithServiceConfig(jsonConfig string)` - Sets gRPC service configuration
- `rpc.WithDescription(description string)` - Adds service documentation

## Method Registration

### Type-Safe Registration (Recommended)

#### `rpc.MustRegisterTyped[TIn, TOut](svc *Service, name string, handler Handler[TIn, TOut])`

The simplest and most type-safe way to register methods:

```go
// Handler with explicit types
func createUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
    // implementation
}

// Register with automatic type inference
rpc.MustRegisterTyped(svc, "CreateUser", createUser)
```

### Method Builder API

#### `rpc.NewMethod[TIn, TOut](name string, handler Handler[TIn, TOut]) *MethodBuilder`

Creates a new method builder with type inference:

```go
method := rpc.NewMethod("CreateUser", createUser).
    Validate(true).              // Override validation setting
    WithInterceptors(authInterceptor). // Add method-specific interceptors
    Description("Creates a new user")  // Add method description
```

### Legacy Registration

For cases where you need to explicitly specify types:

```go
// Using MustRegister with multiple methods
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
| `time.Time` | `google.protobuf.Timestamp` | Automatic conversion |
| `time.Duration` | `google.protobuf.Duration` | Automatic conversion |
| `int` (enum) | `enum` | Integer constants become enum values |

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

### Using RPC Error Types

Hyperway provides structured error types with proper code mapping:

```go
func handler(ctx context.Context, req *Request) (*Response, error) {
    // Validation errors (automatic with validation enabled)
    
    // Using RPC error codes
    if !isAuthorized {
        return nil, rpc.NewError(rpc.CodePermissionDenied, "user not authorized")
    }
    
    // Not found
    if !exists {
        return nil, rpc.NewError(rpc.CodeNotFound, "resource not found")
    }
    
    // Add error details
    err := rpc.NewError(rpc.CodeInvalidArgument, "invalid request")
    err.Details = map[string]any{
        "field": "email",
        "reason": "invalid format",
    }
    return nil, err
}
```

### Error Codes

| Code | Description | HTTP Status (Connect) |
|------|-------------|-----------------------|
| `CodeCanceled` | Operation was canceled | 499 |
| `CodeUnknown` | Unknown error | 500 |
| `CodeInvalidArgument` | Invalid argument | 400 |
| `CodeDeadlineExceeded` | Deadline exceeded | 504 |
| `CodeNotFound` | Not found | 404 |
| `CodeAlreadyExists` | Already exists | 409 |
| `CodePermissionDenied` | Permission denied | 403 |
| `CodeResourceExhausted` | Resource exhausted | 429 |
| `CodeFailedPrecondition` | Failed precondition | 412 |
| `CodeAborted` | Aborted | 409 |
| `CodeOutOfRange` | Out of range | 400 |
| `CodeUnimplemented` | Unimplemented | 501 |
| `CodeInternal` | Internal error | 500 |
| `CodeUnavailable` | Unavailable | 503 |
| `CodeDataLoss` | Data loss | 500 |
| `CodeUnauthenticated` | Unauthenticated | 401 |

### Protocol-Specific Error Handling

- **gRPC**: Errors are mapped to standard gRPC status codes
- **Connect RPC**: Errors are returned in Connect error format with appropriate HTTP status codes

## Interceptors

Interceptors allow you to add cross-cutting concerns like logging, authentication, and metrics.

### Built-in Interceptors

```go
// Logging interceptor
loggingInterceptor := &rpc.LoggingInterceptor{
    Logger: log.Default(),
}

// Timeout interceptor
timeoutInterceptor := &rpc.TimeoutInterceptor{
    Timeout: 30 * time.Second,
}

// Recovery interceptor (catches panics)
recoveryInterceptor := &rpc.RecoveryInterceptor{}

// Apply to service
svc := rpc.NewService("Service",
    rpc.WithInterceptors(
        recoveryInterceptor,
        loggingInterceptor,
        timeoutInterceptor,
    ),
)
```

### Custom Interceptor

```go
type AuthInterceptor struct {
    // your fields
}

func (a *AuthInterceptor) Intercept(
    ctx context.Context,
    method string,
    req any,
    handler func(context.Context, any) (any, error),
) (any, error) {
    // Check authentication
    if !isAuthenticated(ctx) {
        return nil, rpc.NewError(rpc.CodeUnauthenticated, "authentication required")
    }
    
    // Call the handler
    return handler(ctx, req)
}
```

### Method-Specific Interceptors

```go
// Add interceptors to specific methods
method := rpc.NewMethod("SecureMethod", handler).
    WithInterceptors(authInterceptor, rateLimitInterceptor)
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
curl -X POST http://localhost:8080/package.ServiceName/MethodName \
  -H "Content-Type: application/json" \
  -d '{"field": "value"}'

# Connect Protocol  
curl -X POST http://localhost:8080/package.ServiceName/MethodName \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"field": "value"}'

# gRPC (requires HTTP/2)
grpcurl -plaintext -d '{"field": "value"}' localhost:8080 package.ServiceName/MethodName
```

### View OpenAPI Spec

```bash
curl http://localhost:8080/openapi.json | jq
```

## Proto Export

Export your service definitions as `.proto` files for cross-language support:

### Using the CLI

```bash
# Export from running service
hyperway proto export --endpoint http://localhost:8080 --output ./proto

# Export as ZIP
hyperway proto export --endpoint http://localhost:8080 --format zip --output api.zip
```

### Programmatic Export

```go
// Get FileDescriptorSet from service
fdset := svc.GetFileDescriptorSet()

// Export to proto files
exporter := proto.NewExporter(proto.DefaultExportOptions())
files, err := exporter.ExportFileDescriptorSet(fdset)
if err != nil {
    log.Fatal(err)
}

// files is map[filename]content
for filename, content := range files {
    os.WriteFile(filepath.Join("proto", filename), []byte(content), 0644)
}
```