# Hyperway

**Build gRPC/Connect services in Go without writing a single `.proto` file.**

Hyperway is an RPC framework that implements the Connect-RPC and gRPC protocols while letting you define your API using Go structs. It eliminates the need for protobuf files while maintaining wire compatibility with standard gRPC and Connect clients for unary RPCs. Under the hood, it leverages [hyperpb](https://github.com/bufbuild/hyperpb-go) for optimized dynamic protobuf parsing with performance comparable to generated code.

## 🚀 Why Hyperway?

### The Traditional Approach
Traditional gRPC/Connect development follows a schema-first approach:
1. Writing `.proto` files
2. Running `protoc` with various plugins
3. Managing generated code
4. Rebuilding when schemas change

While this approach works well for many use cases, it can be cumbersome for rapid prototyping, small services, or teams that prefer working directly with Go types.

### The Hyperway Approach
Hyperway enables true schema-first development without manual `.proto` files:
1. Define your API using Go structs
2. Run your service
3. Export `.proto` files automatically from the running service
4. Share the schema with other teams from day one

This approach combines the benefits of schema-first development (early API contracts, cross-language support) with the convenience of working directly in Go. You can distribute `.proto` files to other teams immediately, enabling parallel development across different languages and platforms.

### How It Works
Hyperway implements gRPC and Connect RPC protocols with dynamic capabilities:
- Generates Protobuf schemas from your Go structs at runtime
- Supports gRPC (Protobuf) and Connect RPC (both Protobuf and JSON)
- Maintains wire compatibility with standard gRPC/Connect clients
- Supports unary RPCs with full protocol compliance

## 📊 Performance

Hyperway is designed with performance in mind:

- Comparable performance to traditional protoc-generated code
- Minimal overhead from dynamic schema generation
- Optimized memory usage with object pooling
- Efficient message parsing using hyperpb

For detailed benchmarks and performance characteristics, see the [benchmark](./benchmark) directory.

## ✨ Features

- 🚫 **No Proto Files**: Define your API using Go structs
- ⚡ **Optimized Performance**: Uses hyperpb for efficient dynamic protobuf parsing
- 🔄 **Multi-Protocol**: Supports gRPC (Protobuf) and Connect RPC (Protobuf and JSON)
- ✅ **Built-in Validation**: Struct tags for input validation
- 🔍 **gRPC Reflection**: Service discovery without proto files
- 📚 **OpenAPI Generation**: Automatic API documentation
- 🛡️ **Type-Safe**: Full Go type safety without code generation
- 📤 **Proto Export**: Generate `.proto` files from your running service
- 🤝 **Protocol Compatible**: Works with any gRPC or Connect client
- 🗜️ **Compression**: Built-in gzip compression for both gRPC and Connect
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

## 🔄 Best of Both Worlds: Code-First and Schema-First

Hyperway enables a unique hybrid approach:

### 1. Start with Code (No `.proto` files)
```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### 2. Export Proto When Needed
```bash
# Generate .proto files from your running service
hyperway proto export --endpoint localhost:8080 --output ./proto
```

### 3. Share With Other Teams
Now you have standard `.proto` files for:
- Client SDK generation
- Cross-language support
- API documentation
- Schema versioning
- Registry upload (BSR, private repos)

This approach gives you rapid development with Go while maintaining full protobuf compatibility.

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

Hyperway provides:

- **Dynamic Schema Generation**: Converts Go types to Protobuf at runtime
- **Efficient Message Handling**: Uses hyperpb for optimized parsing
- **Multi-Protocol Support**: Implements both gRPC and Connect RPC protocols
- **Extensible Design**: Custom interceptors, codecs, and compressors
- **Type Safety**: Full Go type safety without code generation

## 📈 When to Use Hyperway

✅ **Perfect for:**
- Rapid prototyping and development
- Microservices that need quick iteration
- Teams who prefer Go-first development
- Projects where schema flexibility is important
- Services that need multi-protocol support (gRPC + Connect RPC)
- Applications using unary RPCs
- Services that benefit from automatic validation

❌ **Current Limitations:**
- **No streaming support** - Only unary RPCs are supported
- **Go-only service definitions** - Use exported protos for other languages
- **No gRPC-Web support** - Use Connect protocol for browser clients
- **Limited buf curl compatibility** - Some Well-Known Types (Struct, FieldMask) have JSON parsing issues with buf curl
- **Map of Well-Known Types** - `map[string]*structpb.Value` causes runtime panics (implementation limitation)

## 🚀 Production Readiness

**Status: Production-Ready for Unary RPCs**

Hyperway is production-ready for services using unary RPCs with the following assurances:
- ✅ Comprehensive test coverage
- ✅ Battle-tested protocol compliance  
- ✅ Performance optimized with benchmarks
- ✅ Memory-efficient with pooling
- ✅ Thread-safe implementation
- ✅ Clean static analysis (passes all linters)

For streaming RPCs, use traditional gRPC with `.proto` files until streaming support is added.

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

- [ ] Streaming RPC support (server-streaming, client-streaming, bidirectional)
- [ ] Client library for type-safe RPC calls
- [ ] Metrics and tracing integration (OpenTelemetry)
- [ ] More compression algorithms (br, zstd)
- [ ] Plugin system for custom protocols
- [x] Protobuf Editions support (Edition 2023)
- [x] Additional Well-Known Types (Struct, Value, ListValue, FieldMask)

## 🙏 Acknowledgments

- [Connect-RPC](https://connectrpc.com) - Protocol specification and wire format
- [hyperpb](https://github.com/bufbuild/hyperpb-go) - Blazing-fast protobuf parsing with PGO
- [go-playground/validator](https://github.com/go-playground/validator) - Struct validation
- The Go community for inspiration and feedback
