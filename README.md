# Hyperway

**Schema-driven RPC development, redefined for Go.**

Hyperway bridges code-first agility with schema-first discipline. Your Go structs become the single source of truth, dynamically generating Protobuf schemas at runtime. Serve production-ready gRPC and Connect APIs while maintaining the ability to export standard .proto files to share your schema-driven API with any team, any language.

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
Hyperway implements gRPC and Connect RPC protocols with dynamic capabilities:
- Generates Protobuf schemas from your Go structs at runtime
- Supports gRPC (Protobuf) and Connect RPC (both Protobuf and JSON)
- Maintains wire compatibility with standard gRPC/Connect clients
- Supports unary and server-streaming RPCs with full protocol compliance

## 📊 Performance

Hyperway is designed with performance in mind and offers competitive performance compared to connect-go:

### Benchmark Summary
- **Unary RPCs**: Comparable or better performance across protocols
- **Streaming RPCs**: Significantly improved performance and memory efficiency
- **Memory Usage**: Reduced memory consumption, especially for streaming operations

### Key Performance Features
- Dynamic schema generation with caching
- Efficient message parsing using hyperpb
- Buffer pooling to reduce GC pressure
- Optimized streaming with configurable flushing

For detailed benchmarks and performance characteristics, see the [protocol-benchmarks](./protocol-benchmarks) directory.

## ✨ Features

- 📋 **Schema-First**: Go types as your schema definition language
- 📤 **Proto Export**: Generate standard `.proto` files from your running service
- ⚡ **High Performance**: Uses hyperpb for efficient dynamic protobuf parsing
- 🔄 **Multi-Protocol**: Supports gRPC (Protobuf), Connect RPC (Protobuf and JSON), and gRPC-Web
- 🛡️ **Type-Safe**: Full Go type safety with runtime schema generation
- 🤝 **Protocol Compatible**: Works with any gRPC, Connect, or gRPC-Web client
- ✅ **Built-in Validation**: Struct tags for automatic input validation
- 🔍 **gRPC Reflection**: Service discovery with dynamic schemas
- 📚 **OpenAPI Generation**: Automatic API documentation
- 🌐 **Browser Support**: Native gRPC-Web support without proxy
- 🗜️ **Compression**: Built-in gzip compression for all protocols
- 🔁 **Server Streaming**: Full support for server-streaming RPCs
- ⏰ **Well-Known Types**: Support for common Google Well-Known Types (Timestamp, Duration, Empty, Any, Struct, Value, ListValue, FieldMask)
- 🔌 **Custom Interceptors**: Middleware for logging, auth, metrics, etc.
- 📦 **Proto3 Optional**: Full support for optional fields
- 🎯 **Protobuf Editions**: Support for Edition 2023 with features configuration

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
    )
    
    // Register your handlers
    if err := rpc.Register(svc, "CreateUser", createUser); err != nil {
        log.Fatal(err)
    }
    
    // Start serving (supports gRPC and Connect RPC protocols)
    gateway, _ := rpc.NewGateway(svc)
    log.Fatal(http.ListenAndServe(":8080", gateway))
}
```

## 🧪 Testing Your Service

Your service automatically supports multiple protocols:

### Connect RPC (JSON format)
```bash
curl -X POST http://localhost:8080/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
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
```

Now share your schema-driven API with any team:
- Client SDK generation in any language
- API documentation and contracts
- Schema registries (BSR, private repos)
- Standard protobuf tooling compatibility

This hybrid approach delivers the discipline of schema-first design with the agility of Go-native development.

## 🛠️ CLI Tool

```bash
# Export proto files from a running service
hyperway proto export --endpoint http://localhost:8080 --output ./proto

# Export as ZIP archive
hyperway proto export --endpoint http://localhost:8080 --format zip --output api.zip
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
    
    // Create gateway and serve
    gateway, err := rpc.NewGateway(svc)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println("Blog service running on :8080")
    log.Println("- Connect RPC: POST http://localhost:8080/blog.v1.BlogService/CreatePost")
    log.Println("- gRPC: localhost:8080 (with reflection)")
    log.Fatal(http.ListenAndServe(":8080", gateway))
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
gateway, _ := rpc.NewGateway(userSvc, authSvc, adminSvc)
```

### Server Streaming

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/i2y/hyperway/rpc"
)

// Define request/response types
type WatchEventsRequest struct {
    Filter string `json:"filter" validate:"required"`
    Limit  int32  `json:"limit,omitempty"`
}

type Event struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
}

// Service with streaming method
type EventService struct{}

func (s *EventService) WatchEvents(ctx context.Context, req *WatchEventsRequest, stream rpc.ServerStream[*Event]) error {
    // Send events to the client
    for i := 0; i < 10; i++ {
        event := &Event{
            ID:        fmt.Sprintf("event-%d", i),
            Type:      "update",
            Message:   fmt.Sprintf("Event %d matching filter: %s", i, req.Filter),
            Timestamp: time.Now(),
        }
        
        if err := stream.Send(event); err != nil {
            return err
        }
        
        // Simulate real-time events
        time.Sleep(500 * time.Millisecond)
    }
    
    return nil
}

func main() {
    eventService := &EventService{}
    
    // Create service
    svc := rpc.NewService("EventService",
        rpc.WithPackage("events.v1"),
        rpc.WithReflection(true),
    )
    
    // Register server-streaming method
    if err := rpc.RegisterServerStream(svc, "WatchEvents", eventService.WatchEvents); err != nil {
        log.Fatal(err)
    }
    
    // Create gateway and serve
    gateway, err := rpc.NewGateway(svc)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println("Event service with streaming running on :8080")
    log.Fatal(http.ListenAndServe(":8080", gateway))
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

## 🏗️ Architecture

Hyperway implements a schema-driven architecture where:

### Schema-First Philosophy
- **Go Types as Schema Source**: Your structs define the contract, enforced at compile time
- **Runtime Schema Generation**: Dynamic Protobuf generation maintains wire compatibility
- **Single Source of Truth**: No schema duplication between `.proto` files and Go code

### Technical Foundation
- **High-Performance Parsing**: Leverages hyperpb for optimized message handling
- **Multi-Protocol Gateway**: Unified implementation of gRPC, Connect, and gRPC-Web
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
- Applications using unary and server-streaming RPCs
- Systems requiring automatic validation and type safety
- Organizations wanting to share schemas across polyglot teams

❌ **Current Limitations:**
- **Client/Bidi streaming** - Only server-streaming is currently supported
- **Go-only service definitions** - Use exported protos for other languages
- **Limited buf curl compatibility** - Some Well-Known Types (Struct, FieldMask) have JSON parsing issues with buf curl
- **Map of Well-Known Types** - `map[string]*structpb.Value` causes runtime panics (implementation limitation)
- **gRPC streaming compatibility** - gRPC streaming works but may require special handling for protoc-generated clients due to dynamic schema nature

## 🚀 Current Status

Hyperway supports unary and server-streaming RPCs with:
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

For client-streaming and bidirectional streaming RPCs, use traditional gRPC with `.proto` files until full streaming support is added.

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
- [x] Streaming performance optimizations
- [x] Protobuf Editions support (Edition 2023)
- [x] Additional Well-Known Types (Struct, Value, ListValue, FieldMask)
- [x] Buffer pooling and concurrency optimizations

### In Progress 🚧
- [ ] Client-streaming RPC support
- [ ] Bidirectional streaming RPC support

### Planned 📋
- [ ] Metrics and tracing integration (OpenTelemetry)
- [ ] More compression algorithms (br, zstd)
- [ ] Plugin system for custom protocols

## ❓ FAQ

### Q: How does this work with existing proto tooling?
A: Hyperway generates standard Protobuf schemas. Export them as `.proto` files and use any existing tooling - buf, protoc, linters, breaking change detection, etc. Your exported schemas are fully compatible with the entire Protobuf ecosystem.

### Q: Is this suitable for production use?
A: Yes. Hyperway is designed for production workloads with comprehensive testing, performance optimizations, and memory-efficient implementation. The hybrid approach allows teams to maintain the rigor of schema-first design while improving development velocity.

### Q: What about cross-language support?
A: Export your schemas as `.proto` files and generate clients in any language. Hyperway maintains full wire compatibility with standard gRPC and Connect clients, so your services work seamlessly with clients written in any supported language.

### Q: Can I migrate from traditional proto-first development?
A: Yes. You can gradually adopt Hyperway service by service. Existing proto-based services can coexist with Hyperway services in the same system. You can even import existing `.proto` files as a starting point (feature in development).

## 🙏 Acknowledgments

- [Connect-RPC](https://connectrpc.com) - Protocol specification and wire format
- [hyperpb](https://github.com/bufbuild/hyperpb-go) - High-performance protobuf parsing with PGO
- [go-playground/validator](https://github.com/go-playground/validator) - Struct validation
- The Go community for inspiration and feedback
