# Hyperway

**Schema-driven RPC development, redefined for Go.**

Hyperway bridges code-first agility with schema-first discipline. Your Go structs become the single source of truth, dynamically generating Protobuf schemas at runtime. Serve production-ready gRPC, Connect, gRPC-Web, and JSON-RPC 2.0 APIs from a single codebase, with automatic OpenAPI documentation, while maintaining the ability to export standard .proto files to share your schema-driven API with any team, any language.

## 🚀 Why Hyperway?

### The Traditional Approach
Traditional gRPC/Connect development follows a schema-first approach:
1. Writing `.proto` files
2. Running `protoc` with various plugins
3. Managing generated code
4. Rebuilding when schemas change

While this approach works well for many use cases, it can be cumbersome for rapid prototyping, small services, or teams that prefer working directly with Go types.

### Benefits of Traditional Proto-First Development

The traditional approach offers important advantages:
- **Language-neutral contracts** - `.proto` files serve as universal API documentation
- **Mature tooling ecosystem** - Linters, breaking change detection, versioning tools
- **Clear team boundaries** - Explicit contracts for cross-team collaboration
- **Established workflows** - Well-understood CI/CD patterns

### The Hyperway Approach
Hyperway preserves these benefits while accelerating development:
1. Define your API using Go structs - your types are the schema
2. Run your service with automatic schema generation
3. Export `.proto` files whenever needed for cross-team collaboration
4. Use all existing proto tooling with your exported schemas

This hybrid approach maintains the discipline of schema-first development while removing friction from the development cycle. Teams can work rapidly in Go while still providing standard `.proto` files for tooling, documentation, and cross-language support.

### How It Works
Hyperway implements multiple RPC protocols with dynamic capabilities:
- Generates Protobuf schemas from your Go structs at runtime
- Supports gRPC (Protobuf), Connect RPC (both Protobuf and JSON), gRPC-Web, and JSON-RPC 2.0
- Automatically generates OpenAPI 3.0 documentation at `/openapi.json`
- Maintains wire compatibility with standard clients for all protocols
- Supports all RPC types: unary, server-streaming, client-streaming, and bidirectional streaming
- Handles both HTTP/1.1 and HTTP/2 (with h2c support)

## 📊 Performance

Hyperway is designed with performance in mind and offers competitive performance compared to connect-go:

### Benchmark Summary
- **Unary RPCs**: Comparable performance across protocols
- **Server Streaming**: Improved performance and memory efficiency
- **Client Streaming**: Competitive performance
- **Bidirectional Streaming**: Efficient implementation
- **Memory Usage**: Reduced memory consumption for streaming operations

### Key Performance Features
- Dynamic schema generation with caching
- Efficient message parsing using hyperpb
- Buffer pooling to reduce GC pressure
- Optimized streaming with configurable flushing

For detailed benchmarks and performance characteristics, see the [protocol-benchmarks](./protocol-benchmarks) directory.

## ✨ Features

- 📋 **Schema-First**: Go types as your schema definition language
- 📤 **Proto Export**: Generate standard `.proto` files with language-specific options
- ⚡ **High Performance**: Uses hyperpb for efficient dynamic protobuf parsing
- 🔄 **Multi-Protocol**: Supports gRPC, Connect RPC, gRPC-Web, and JSON-RPC 2.0 on the same server
- 🛡️ **Type-Safe**: Full Go type safety with runtime schema generation
- 🤝 **Protocol Compatible**: Works with any gRPC, Connect, or gRPC-Web client
- ✅ **Built-in Validation**: Struct tags for automatic input validation
- 🔍 **gRPC Reflection**: Service discovery with dynamic schemas
- 📚 **OpenAPI Generation**: Automatic API documentation
- 🌐 **Browser Support**: Native gRPC-Web support without proxy
- 🗜️ **Compression**: Multi-algorithm support (gzip, brotli, zstd) for all protocols
- 🔁 **All Streaming Types**: Support for server, client, and bidirectional streaming RPCs
- ⏰ **Well-Known Types**: Support for common Google Well-Known Types (Timestamp, Duration, Empty, Any, Struct, Value, ListValue, FieldMask)
- 🔌 **Custom Interceptors**: Middleware for logging, auth, metrics, etc.
- 📦 **Proto3 Optional**: Full support for optional fields
- 🎯 **Protobuf Editions**: Support for Edition 2023 with features configuration
- 📏 **Message Size Limits**: Configurable max send/receive message sizes

## 📦 Installation

```bash
# Library
go get github.com/i2y/hyperway

# CLI tool
go install github.com/i2y/hyperway/cmd/hyperway@latest
```

## 🎯 Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    
    "github.com/i2y/hyperway/rpc"
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

// Define your API using Go structs
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// Write your business logic
func createUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
    // Your business logic here
    return &CreateUserResponse{
        ID:   "user-123",
        Name: req.Name,
    }, nil
}

func main() {
    // Create a service
    svc := rpc.NewService("UserService", 
        rpc.WithPackage("user.v1"),
        rpc.WithValidation(true),
        rpc.WithMaxReceiveMessageSize(10 * 1024 * 1024), // 10MB max receive
        rpc.WithMaxSendMessageSize(10 * 1024 * 1024),    // 10MB max send
    )
    
    // Register your handlers
    if err := rpc.Register(svc, "CreateUser", createUser); err != nil {
        log.Fatal(err)
    }
    
    // Create handler - returns a standard http.Handler
    handler, _ := rpc.NewHandler(svc)
    
    // Wrap with h2c to support both HTTP/1.1 and HTTP/2 (required for gRPC)
    h2s := &http2.Server{}
    h2Handler := h2c.NewHandler(handler, h2s)
    
    // Serve (now supports all protocols including gRPC over HTTP/2)
    log.Fatal(http.ListenAndServe(":8080", h2Handler))
}
```

### 🔧 Protocol Configuration

By default, Hyperway enables Connect, gRPC, and gRPC-Web protocols. You can customize this:

```go
// Enable JSON-RPC in addition to defaults
svc := rpc.NewService("UserService",
    rpc.WithProtocols(
        rpc.WithDefaults(rpc.JSONRPC("/api/jsonrpc"))...,
    ),
)

// Use specific protocols only
svc := rpc.NewService("UserService",
    rpc.WithProtocols(
        rpc.Connect(),
        rpc.JSONRPC("/api/jsonrpc"),
    ),
)

// Disable specific protocols
svc := rpc.NewService("UserService",
    rpc.WithoutGRPCWeb(),  // Disable gRPC-Web
)
```

## 🧪 Testing Your Service

Your service automatically supports multiple protocols and provides OpenAPI documentation:

### Connect RPC (JSON format)
```bash
curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

### JSON-RPC 2.0
```bash
# Note: Requires JSON-RPC to be enabled with WithProtocols()
curl -X POST http://localhost:8080/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "UserService.CreateUser",
    "params": {"name":"Alice","email":"alice@example.com"},
    "id": 1
  }'
```

### gRPC (with reflection)
```bash
grpcurl -plaintext -d '{"name":"Bob","email":"bob@example.com"}' \
  localhost:8080 user.v1.UserService/CreateUser
```

### Connect Protocol (JSON)
```bash
curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name":"Charlie","email":"charlie@example.com"}'
```

### Connect Protocol (Protobuf)
```bash
# Using buf curl for Connect protocol testing
buf curl --protocol connect \
  --http2-prior-knowledge \
  --data '{"name":"David","email":"david@example.com"}' \
  http://localhost:8080/user.v1.UserService/CreateUser
```

### OpenAPI Documentation
```bash
# Get OpenAPI 3.0 specification
curl http://localhost:8080/openapi.json

# View in Swagger UI or any OpenAPI viewer
# The spec includes all your RPC methods with request/response schemas
```

## 🔄 The Hybrid Approach: Schema-Driven Development in Go

Hyperway redefines schema-driven development for the Go ecosystem:

### 1. Define Your Schema in Go
```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```
Your Go types ARE the schema - type-safe, validated, and version-controlled with your code.

### 2. Runtime Schema Generation
Hyperway automatically generates Protobuf schemas from your types at runtime, maintaining full wire compatibility with standard gRPC/Connect clients.

### 3. Export Schemas for Cross-Team Collaboration
```bash
# Generate standard .proto files from your running service
hyperway proto export --endpoint localhost:8080 --output ./proto

# Export with language-specific options (no manual editing needed!)
hyperway proto export --endpoint localhost:8080 \
  --go-package "github.com/example/api;apiv1" \
  --java-package "com.example.api"
```

Now share your schema-driven API with any team:
- Client SDK generation in any language
- API documentation and contracts
- Language-specific options are automatically added
- Schema registries (BSR, private repos)
- Standard protobuf tooling compatibility

This hybrid approach delivers the discipline of schema-first design with the agility of Go-native development.

## 🛠️ CLI Tool

```bash
# Export proto files from a running service
hyperway proto export --endpoint http://localhost:8080 --output ./proto

# Export with language-specific options (no manual editing needed!)
hyperway proto export --endpoint http://localhost:8080 \
  --go-package "github.com/example/api;apiv1" \
  --java-package "com.example.api" \
  --csharp-namespace "Example.Api"

# Export as ZIP archive with options
hyperway proto export --endpoint http://localhost:8080 \
  --format zip --output api.zip \
  --go-package "github.com/example/api;apiv1"

# See all available language options
hyperway proto export --help
```

## 📚 Advanced Usage

### Complex Types

Hyperway supports all Go types you need:

```go
type Order struct {
    ID        string                 `json:"id"`
    Items     []OrderItem           `json:"items"`
    Metadata  map[string]string     `json:"metadata"`
    Customer  *Customer             `json:"customer,omitempty"`
    Status    OrderStatus           `json:"status"`
    CreatedAt time.Time             `json:"created_at"`
}
```

### Well-Known Types

Hyperway supports the most commonly used Google Well-Known Types:

```go
import (
    "google.golang.org/protobuf/types/known/structpb"
    "google.golang.org/protobuf/types/known/fieldmaskpb"
)

type UpdateRequest struct {
    // Dynamic configuration using Struct
    Config *structpb.Struct `json:"config"`
    
    // Partial updates using FieldMask
    UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"`
    
    // Mixed-type values
    Settings map[string]*structpb.Value `json:"settings"`
}
```

### Validation

Use struct tags for automatic validation:

```go
type RegisterRequest struct {
    Username string `json:"username" validate:"required,alphanum,min=3,max=20"`
    Password string `json:"password" validate:"required,min=8,containsany=!@#$%"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"required,min=13,max=120"`
}
```

### Real-World Example

Here's a more complete example showing various features:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/i2y/hyperway/rpc"
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

// Domain models with validation and well-known types
type CreatePostRequest struct {
    Title     string    `json:"title" validate:"required,min=5,max=200"`
    Content   string    `json:"content" validate:"required,min=10"`
    AuthorID  string    `json:"author_id" validate:"required,uuid"`
    Tags      []string  `json:"tags" validate:"max=10,dive,min=2,max=20"`
    Published bool      `json:"published"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}

type Post struct {
    ID          string            `json:"id"`
    Title       string            `json:"title"`
    Content     string            `json:"content"`
    AuthorID    string            `json:"author_id"`
    Tags        []string          `json:"tags"`
    Published   bool              `json:"published"`
    PublishedAt *time.Time        `json:"published_at,omitempty"` // Optional timestamp
    CreatedAt   time.Time         `json:"created_at"`             // Required timestamp
    UpdatedAt   time.Time         `json:"updated_at"`
    TTL         *time.Duration    `json:"ttl,omitempty"`          // Optional duration
    Metadata    map[string]string `json:"metadata"`
}

// Service implementation
type BlogService struct {
    // your database, cache, etc.
}

func (s *BlogService) CreatePost(ctx context.Context, req *CreatePostRequest) (*Post, error) {
    // Business logic here
    now := time.Now()
    post := &Post{
        ID:        generateID(),
        Title:     req.Title,
        Content:   req.Content,
        AuthorID:  req.AuthorID,
        Tags:      req.Tags,
        Published: req.Published,
        CreatedAt: now,
        UpdatedAt: now,
        Metadata:  req.Metadata,
    }
    
    if req.Published {
        post.PublishedAt = &now
        ttl := 30 * 24 * time.Hour // 30 days
        post.TTL = &ttl
    }
    
    // Save to database...
    
    return post, nil
}

func main() {
    // Create blog service
    blogService := &BlogService{}
    
    // Create RPC service with interceptors
    svc := rpc.NewService("BlogService",
        rpc.WithPackage("blog.v1"),
        rpc.WithValidation(true),
        rpc.WithReflection(true),
        rpc.WithInterceptor(&rpc.RecoveryInterceptor{}),
        rpc.WithInterceptor(&rpc.TimeoutInterceptor{Timeout: 30*time.Second}),
    )
    
    // Register methods - no need to specify types!
    if err := rpc.Register(svc, "CreatePost", blogService.CreatePost); err != nil {
        log.Fatal(err)
    }
    
    // Create handler and serve
    handler, err := rpc.NewHandler(svc)
    if err != nil {
        log.Fatal(err)
    }
    
    // Wrap with h2c for HTTP/2 support (required for gRPC)
    h2s := &http2.Server{}
    h2Handler := h2c.NewHandler(handler, h2s)
    
    log.Println("Blog service running on :8080")
    log.Println("- Connect RPC: POST http://localhost:8080/blog.v1.BlogService/CreatePost")
    log.Println("- gRPC: localhost:8080 (with reflection)")
    log.Fatal(http.ListenAndServe(":8080", h2Handler))
}
```

### Multiple Services

```go
// Create multiple services
userSvc := rpc.NewService("UserService", rpc.WithPackage("api.v1"))
authSvc := rpc.NewService("AuthService", rpc.WithPackage("api.v1"))
adminSvc := rpc.NewService("AdminService", rpc.WithPackage("api.v1"))

// Register handlers
rpc.Register(userSvc, "CreateUser", createUser)
rpc.Register(userSvc, "GetUser", getUser)
rpc.Register(authSvc, "Login", login)
rpc.Register(adminSvc, "DeleteUser", deleteUser)

// Serve all services on one port
handler, _ := rpc.NewHandler(userSvc, authSvc, adminSvc)
```

### Streaming RPCs

#### Server Streaming

```go
// Server sends multiple responses to client
func (s *Service) WatchEvents(ctx context.Context, req *WatchRequest, stream rpc.ServerStream[*Event]) error {
    for i := 0; i < 10; i++ {
        event := &Event{
            ID:      fmt.Sprintf("event-%d", i),
            Message: fmt.Sprintf("Event %d", i),
        }
        if err := stream.Send(event); err != nil {
            return err
        }
        time.Sleep(100 * time.Millisecond)
    }
    return nil
}

// Register: rpc.RegisterServerStream(svc, "WatchEvents", service.WatchEvents)
```

#### Client Streaming

```go
// Client sends multiple requests, server sends single response
func (s *Service) UploadFiles(ctx context.Context, stream rpc.ClientStream[*FileChunk]) (*UploadResult, error) {
    var totalSize int64
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
        totalSize += int64(len(chunk.Data))
    }
    return &UploadResult{TotalSize: totalSize}, nil
}

// Register: rpc.RegisterClientStream(svc, "UploadFiles", service.UploadFiles)
```

#### Bidirectional Streaming

```go
// Both client and server send multiple messages
func (s *Service) Chat(ctx context.Context, stream rpc.BidiStream[*ChatMessage, *ChatResponse]) error {
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        response := &ChatResponse{
            Message: fmt.Sprintf("Echo: %s", msg.Text),
        }
        if err := stream.Send(response); err != nil {
            return err
        }
    }
}

// Register: rpc.RegisterBidiStream(svc, "Chat", service.Chat)
```

#### Complete Example with All Streaming Types

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
    
    "github.com/i2y/hyperway/rpc"
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

// Request/Response types
type WatchRequest struct {
    Filter string `json:"filter"`
}

type Event struct {
    ID      string    `json:"id"`
    Message string    `json:"message"`
    Time    time.Time `json:"time"`
}

type FileChunk struct {
    Name string `json:"name"`
    Data []byte `json:"data"`
}

type UploadResult struct {
    TotalSize int64 `json:"total_size"`
}

type ChatMessage struct {
    Text string `json:"text"`
}

type ChatResponse struct {
    Message string `json:"message"`
}

// Service implementation
type StreamService struct{}

// Server streaming
func (s *StreamService) WatchEvents(ctx context.Context, req *WatchRequest, stream rpc.ServerStream[*Event]) error {
    for i := 0; i < 5; i++ {
        event := &Event{
            ID:      fmt.Sprintf("event-%d", i),
            Message: fmt.Sprintf("Filtered by: %s", req.Filter),
            Time:    time.Now(),
        }
        if err := stream.Send(event); err != nil {
            return err
        }
        time.Sleep(100 * time.Millisecond)
    }
    return nil
}

// Client streaming
func (s *StreamService) UploadFiles(ctx context.Context, stream rpc.ClientStream[*FileChunk]) (*UploadResult, error) {
    var totalSize int64
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
        totalSize += int64(len(chunk.Data))
    }
    return &UploadResult{TotalSize: totalSize}, nil
}

// Bidirectional streaming
func (s *StreamService) Chat(ctx context.Context, stream rpc.BidiStream[*ChatMessage, *ChatResponse]) error {
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        response := &ChatResponse{
            Message: fmt.Sprintf("Server says: %s", msg.Text),
        }
        if err := stream.Send(response); err != nil {
            return err
        }
    }
}

func main() {
    service := &StreamService{}
    
    // Create service
    svc := rpc.NewService("StreamService",
        rpc.WithPackage("stream.v1"),
        rpc.WithReflection(true),
    )
    
    // Register all streaming methods
    rpc.MustRegisterServerStream(svc, "WatchEvents", service.WatchEvents)
    rpc.MustRegisterClientStream(svc, "UploadFiles", service.UploadFiles)
    rpc.MustRegisterBidiStream(svc, "Chat", service.Chat)
    
    // Create handler and serve
    handler, err := rpc.NewHandler(svc)
    if err != nil {
        log.Fatal(err)
    }
    
    // Wrap with h2c for HTTP/2 support (required for gRPC)
    h2s := &http2.Server{}
    h2Handler := h2c.NewHandler(handler, h2s)
    
    log.Println("Streaming service running on :8080")
    log.Fatal(http.ListenAndServe(":8080", h2Handler))
}
```

### Advanced Registration (Optional)

For more control, you can use the builder pattern:

```go
// Use the builder pattern for additional options
rpc.MustRegisterMethod(svc,
    rpc.NewMethod("CreateUser", createUser).
        Validate(true).
        WithInterceptors(customInterceptor),
)
```

### Interceptors/Middleware

```go
// Add logging, auth, rate limiting, etc.
svc := rpc.NewService("MyService",
    rpc.WithInterceptor(&rpc.LoggingInterceptor{}),
    rpc.WithInterceptor(&rpc.RecoveryInterceptor{}),
)
```

### HTTP Middleware and Handler Composition

Hyperway's handler implements the standard `http.Handler` interface, making it fully compatible with Go's HTTP ecosystem. This means you can:
- Use any standard net/http middleware
- Combine it with other HTTP handlers
- Integrate with existing HTTP routers and frameworks
- **Pass context values from middleware to RPC handlers**

#### Context Propagation

HTTP middleware can add values to the request context, and these values will be accessible in your RPC handlers:

```go
// Middleware that adds context values
func contextMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add request ID
        ctx := context.WithValue(r.Context(), "request-id", generateRequestID())
        
        // Add user info from auth header
        if userID := extractUserID(r.Header.Get("Authorization")); userID != "" {
            ctx = context.WithValue(ctx, "user-id", userID)
        }
        
        // Pass the enriched context to the next handler
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// RPC handler can access context values
func createOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
    // Get values from context
    requestID, _ := ctx.Value("request-id").(string)
    userID, _ := ctx.Value("user-id").(string)
    
    log.Printf("Processing order for user %s (request: %s)", userID, requestID)
    
    // Your business logic here...
    return &CreateOrderResponse{
        OrderID:   generateOrderID(),
        RequestID: requestID,
    }, nil
}

// Logging middleware with request tracking
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        requestID := generateRequestID()
        
        // Add request ID to context for correlation
        ctx := context.WithValue(r.Context(), "request-id", requestID)
        
        log.Printf("[%s] Started %s %s", requestID, r.Method, r.URL.Path)
        
        // Wrap ResponseWriter to capture status
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        next.ServeHTTP(wrapped, r.WithContext(ctx))
        
        log.Printf("[%s] Completed with %d in %v", 
            requestID, wrapped.statusCode, time.Since(start))
    })
}

// Auth middleware with context enrichment
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // Validate token and extract user info
        userInfo, err := validateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Add user info to context
        ctx := context.WithValue(r.Context(), "user", userInfo)
        ctx = context.WithValue(ctx, "user-id", userInfo.ID)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

```

#### Complete Example

Here's a complete example showing middleware composition and context propagation:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/i2y/hyperway/rpc"
    "github.com/google/uuid"
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

// Request/Response types
type CreateOrderRequest struct {
    ProductID string `json:"product_id" validate:"required"`
    Quantity  int    `json:"quantity" validate:"required,min=1"`
}

type CreateOrderResponse struct {
    OrderID   string `json:"order_id"`
    UserID    string `json:"user_id"`
    RequestID string `json:"request_id"`
}

// Business logic that uses context values
func createOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
    // Access context values set by middleware
    requestID, _ := ctx.Value("request-id").(string)
    userID, _ := ctx.Value("user-id").(string)
    
    log.Printf("[%s] Creating order for user %s: %d x %s", 
        requestID, userID, req.Quantity, req.ProductID)
    
    // Create order...
    orderID := fmt.Sprintf("order-%s", uuid.New().String()[:8])
    
    return &CreateOrderResponse{
        OrderID:   orderID,
        UserID:    userID,
        RequestID: requestID,
    }, nil
}

func main() {
    // Create service
    svc := rpc.NewService("OrderService",
        rpc.WithPackage("shop.v1"),
        rpc.WithValidation(true),
    )
    
    // Register handlers
    rpc.Register(svc, "CreateOrder", createOrder)
    
    // Create handler
    handler, _ := rpc.NewHandler(svc)
    
    // Create a standard mux
    mux := http.NewServeMux()
    
    // Chain middleware: auth -> logging -> context -> handler
    // Context values flow through to RPC handlers
    mux.Handle("/", 
        authMiddleware(
            loggingMiddleware(
                contextMiddleware(handler))))
    
    // Add health check endpoint
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, `{"status":"healthy"}`)
    })
    
    // Serve static files
    mux.Handle("/static/", http.StripPrefix("/static/", 
        http.FileServer(http.Dir("./static"))))
    
    // Add metrics endpoint
    mux.Handle("/metrics", promhttp.Handler())
    
    // Use with popular routers (e.g., gorilla/mux, chi)
    // router := chi.NewRouter()
    // router.Use(middleware.RequestID)
    // router.Use(middleware.Logger)
    // router.Mount("/api", handler)
    
    // Wrap with h2c for HTTP/2 support
    h2s := &http2.Server{}
    h2Handler := h2c.NewHandler(mux, h2s)
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", h2Handler))
}
```

This flexibility allows you to:
- **Pass request-scoped data** (request ID, user info, trace ID) from middleware to handlers
- **Add authentication, rate limiting, or CORS handling** at the HTTP layer
- **Serve your RPC API alongside REST endpoints** on the same server
- **Integrate with observability tools** (Prometheus, OpenTelemetry)
- **Use popular Go web frameworks and routers** (chi, gin, echo, gorilla/mux)
- **Implement custom request/response processing** with full HTTP control

The key insight is that Hyperway's handler is just a standard `http.Handler`, so any context values set via `r.WithContext()` in your middleware will be available in your RPC handlers via the `ctx` parameter.

## 🏗️ Architecture

Hyperway implements a schema-driven architecture where:

### Schema-First Philosophy
- **Go Types as Schema Source**: Your structs define the contract, enforced at compile time
- **Runtime Schema Generation**: Dynamic Protobuf generation maintains wire compatibility
- **Single Source of Truth**: No schema duplication between `.proto` files and Go code

### Technical Foundation
- **High-Performance Parsing**: Leverages hyperpb for optimized message handling
- **Multi-Protocol Gateway**: Unified implementation of gRPC, Connect, and gRPC-Web
- **Standard http.Handler Interface**: Seamless integration with Go's HTTP ecosystem
- **Extensible Middleware**: Interceptors for cross-cutting concerns
- **Type-Safe by Design**: Compile-time type checking with runtime protocol compliance

## 🔄 Hyperway vs Traditional Development

### Development Workflow Comparison

**Traditional Proto-First:**
1. Edit `.proto` file
2. Run code generation
3. Update implementation
4. Handle generated code inconsistencies

**Hyperway:**
1. Edit Go struct
2. Run service
3. (Optional) Export `.proto` when sharing

### When to Export Protos

Export `.proto` files when you need:
- **Cross-language clients** - Generate SDKs for other languages
- **API documentation** - Share contracts with external teams
- **Breaking change detection** - Use with buf or similar tools
- **Schema registries** - Upload to BSR or internal registries

### Complementary Workflow

```bash
# Development phase: Iterate rapidly with Go types
# Just write code, test, and refine

# Collaboration phase: Export schemas for wider use
hyperway proto export --endpoint localhost:8080 --output ./proto

# Now you have both:
# - Fast iteration for ongoing development
# - Standard .proto files for tooling and cross-team collaboration
```

## 📈 When to Use Hyperway

✅ **Perfect for:**
- Teams embracing schema-driven development with Go
- Microservices requiring both type safety and rapid iteration
- Projects that value schema-first principles without manual schema maintenance
- Services that need multi-protocol support (gRPC + Connect RPC)
- Applications using all RPC types (unary, server/client/bidirectional streaming)
- Systems requiring automatic validation and type safety
- Organizations wanting to share schemas across polyglot teams

❌ **Current Limitations:**
- **Go-only service definitions** - Use exported protos for other languages
- **Limited buf curl compatibility** - Some Well-Known Types (Struct, FieldMask) have JSON parsing issues with buf curl
- **Map of Well-Known Types** - `map[string]*structpb.Value` causes runtime panics (implementation limitation)
- **gRPC streaming compatibility** - All streaming types (server/client/bidirectional) work with standard gRPC and Connect clients

## 🚀 Current Status

Hyperway supports all RPC types (unary, server-streaming, client-streaming, and bidirectional streaming) with:
- ✅ Comprehensive test coverage
- ✅ Performance optimizations
- ✅ Memory-efficient implementation
- ✅ Thread-safe design
- ✅ Clean static analysis
- ✅ Configurable streaming behavior

### Tooling Integration
- ✅ **Proto Export** - Generate standard `.proto` files from running services
- ✅ **Full Compatibility** - Exported protos work with buf, protoc, and all standard tools
- ✅ **Schema Registries** - Compatible with BSR and corporate registries
- ✅ **Wire Compatibility** - Works with any gRPC/Connect client


## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/i2y/hyperway.git
cd hyperway

# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint

# Run benchmarks
make bench
```

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🗺️ Roadmap

### Completed ✅
- [x] Server-streaming RPC support
- [x] Client-streaming RPC support
- [x] Bidirectional streaming RPC support
- [x] Streaming performance optimizations
- [x] Protobuf Editions support (Edition 2023)
- [x] Additional Well-Known Types (Struct, Value, ListValue, FieldMask)
- [x] Buffer pooling and concurrency optimizations
- [x] JSON-RPC 2.0 protocol support (v0.4.0)
- [x] Multi-compression support: gzip, brotli, zstd (v0.5.0)
- [x] Configurable message size limits (v0.5.0)
- [x] Enhanced proto export with language-specific options (v0.5.0)
- [x] Unified protocol configuration API (v0.6.0)

### In Progress 🚧

### Planned 📋
- [ ] Metrics and tracing integration (OpenTelemetry)
- [ ] WebSocket support for JSON-RPC
- [ ] Plugin system for custom protocols

## ❓ FAQ

### Q: How does this work with existing proto tooling?
A: Hyperway generates standard Protobuf schemas. Export them as `.proto` files and use any existing tooling - buf, protoc, linters, breaking change detection, etc. Your exported schemas are fully compatible with the entire Protobuf ecosystem.

### Q: Is this suitable for production use?
A: Hyperway is currently under active development but may be suitable for production use depending on your requirements. It supports all RPC types (unary, server-streaming, client-streaming, and bidirectional streaming) with competitive performance. We recommend thoroughly testing it for your specific use case before production deployment. It's particularly well-suited for prototyping, internal tools, and services where rapid development is prioritized.

### Q: What about cross-language support?
A: Export your schemas as `.proto` files and generate clients in any language. Hyperway maintains full wire compatibility with standard gRPC and Connect clients, so your services work seamlessly with clients written in any supported language.

## 🙏 Acknowledgments

- [Connect-RPC](https://connectrpc.com) - Protocol specification and wire format
- [hyperpb](https://github.com/bufbuild/hyperpb-go) - High-performance protobuf parsing with PGO
- [go-playground/validator](https://github.com/go-playground/validator) - Struct validation
- The Go community for inspiration and feedback
