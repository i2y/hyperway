# Supported Features

This document provides a comprehensive list of features currently implemented in Hyperway.

## ✅ Core RPC Features
- **Unary RPC** (request/response)
- **Runtime Protobuf schema generation**
- **Type-safe handler registration**
- **Multiple services on single port**

## ✅ Data Type Support

### Basic Types (Fully Supported)
```go
string   → protobuf string
bool     → protobuf bool  
int32    → protobuf int32
int64    → protobuf int64
uint32   → protobuf uint32
uint64   → protobuf uint64
float32  → protobuf float
float64  → protobuf double
[]byte   → protobuf bytes
```

### Complex Types
```go
// ✅ Supported
[]T                    // slice → repeated
map[string]T           // map → map<string, T>
map[int32]T            // integer key maps
*T                     // pointer → proto3 optional
struct                 // struct → message
*struct                // struct pointer → optional message

// ❌ Not Supported
[]*T                   // slice of pointers
map[K]*V               // map with pointer values
interface{}            // interfaces
chan T                 // channels
func                   // function types
```

### Google Well-Known Types
```go
// ✅ Automatic Conversion Support
time.Time              // → google.protobuf.Timestamp
*time.Time             // → optional google.protobuf.Timestamp
time.Duration          // → google.protobuf.Duration
*time.Duration         // → optional google.protobuf.Duration
struct{}               // → google.protobuf.Empty
field `proto:"empty"`  // → google.protobuf.Empty

// ❌ Not Yet Supported
google.protobuf.Any
google.protobuf.Struct
google.protobuf.Value
google.protobuf.ListValue
Wrapper types (StringValue, Int32Value, etc.)
```

## ✅ Protocol Support
- **gRPC** - Full gRPC protocol over HTTP/2
- **Connect** - Connect-RPC protocol
- **REST/JSON** - Standard HTTP JSON API
- **Automatic protocol detection** - Based on headers

## ✅ Validation
```go
// Supported validation tags
type Request struct {
    Name  string `validate:"required,min=3,max=50"`
    Email string `validate:"required,email"`
    Age   int32  `validate:"min=0,max=150"`
    URL   string `validate:"url"`
}
```

## ✅ Service Discovery
- **gRPC Server Reflection** - Compatible with grpcurl
- **OpenAPI 3.0 generation** - Automatic API spec
- **buf curl support** - Compatible with buf toolchain

## ✅ Performance Features
- **hyperpb integration** - 10x faster dynamic parsing
- **Schema caching** - Reuse for performance
- **PGO (Profile-Guided Optimization)** - Profile-based optimization
- **Message pooling** - sync.Pool for memory efficiency
- **Buffer pooling** - Fast buffer reuse
- **Handler caching** - Pre-computed metadata
- **Compression support** - gzip for both gRPC/Connect

## ✅ Developer Experience
- **No .proto files** - Pure Go struct definitions
- **JSON tag support** - Field name control
- **Fluent API** - Method chaining
- **Automatic error mapping** - Protocol-specific errors
- **Proto file export** - Generate .proto from running service
- **Custom interceptors** - Middleware pattern
- **proto3 optional** - Automatic pointer type mapping

## ❌ Not Supported

### Streaming RPC
- Server streaming
- Client streaming  
- Bidirectional streaming
- **Reason**: Unary RPC only in current implementation

### Advanced Protobuf Features
- Proto2 syntax
- Protobuf Extensions
- Custom options (partial)
- Some Well-Known Types (Any, Struct, Value, etc.)

### Other Limitations
- gRPC-Web (requires additional proxy)
- Message mutations (hyperpb is read-only)
- Circular references
- Direct generic type usage

## ✅ Oneof Fields

Hyperway supports oneof fields using explicit tagging:

```go
// Struct embedding + hyperway:"oneof" tag
type UpdateRequest struct {
    UserID string
    
    // oneof identifier - explicitly tagged
    Identifier struct {
        Email       *string
        PhoneNumber *string  
        Username    *string
    } `hyperway:"oneof"`
}

// Usage
req := UpdateRequest{
    UserID: "123",
}
req.Identifier.Email = ptr("user@example.com")
// req.Identifier.PhoneNumber = ptr("...") // Error: only one field allowed

// JSON representation
{
  "user_id": "123",
  "identifier": {
    "email": "user@example.com"
  }
}
```

Generated Protobuf:
```protobuf
message UpdateRequest {
  string user_id = 1;
  
  oneof identifier {
    string email = 2;
    string phone_number = 3;
    string username = 4;
  }
}
```

## 📊 Performance Characteristics

| Metric | Measurement | Comparison |
|--------|-------------|------------|
| Basic HTTP Handler | ~38.2μs/req | Baseline |
| Hyperway (full features) | ~44.4μs/req | +6.2μs (16% increase) |
| Connect-Go (typical) | ~40.0μs/req | +1.8μs (4.5% increase) |
| Schema generation | ~53ns/op | First time only |
| Message processing | ~0.23ns/op | Optimized |
| Memory allocation | ~30ns/op | With pooling |

## 🚀 Production Readiness

**Status: Production-Ready for Unary RPC**

Quality assurances:
- ✅ Comprehensive test coverage
- ✅ Protocol compatibility verified
- ✅ Performance optimized
- ✅ Memory efficient
- ✅ Thread-safe
- ✅ Static analysis passing (all linters)
- ✅ Race condition tests passing

## 📋 Recommended Use Cases

### Ideal For
- ✅ Production Unary RPC services
- ✅ Rapid prototyping
- ✅ Internal APIs/microservices
- ✅ Frequently changing schemas
- ✅ Development speed focused projects
- ✅ Multi-protocol services

### Limitations
- ❌ Streaming RPC (not implemented)
- ❌ Some Well-Known Types (Any, Struct, etc.)
- ⚠️ Consider performance impact for extremely high-throughput services

## ✅ Verified

All features verified with:
- `/test/comprehensive_test.go` - Unit tests
- `/test/demo_server.go` - Integration demo
- `/benchmark/` - Performance benchmarks
