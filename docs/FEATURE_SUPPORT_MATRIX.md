# Hyperway Feature Support Matrix

## ✅ Fully Supported Features

### Core RPC Functionality
- ✅ **Unary RPC** - Single request/response pattern
- ✅ **Dynamic Schema Generation** - Runtime protobuf schema from Go structs
- ✅ **Method Registration** - Fluent API for registering RPC methods
- ✅ **Service Grouping** - Multiple services in single handler

### Type System Support
#### Basic Types
- ✅ `string` → `string`
- ✅ `bool` → `bool`
- ✅ `int32` → `int32`
- ✅ `int64`, `int` → `int64`
- ✅ `uint32` → `uint32`
- ✅ `uint64`, `uint` → `uint64`
- ✅ `float32` → `float`
- ✅ `float64` → `double`
- ✅ `[]byte` → `bytes`
- ✅ `time.Time` → `google.protobuf.Timestamp`
- ✅ `time.Duration` → `google.protobuf.Duration`

#### Complex Types
- ✅ **Slices/Arrays** → `repeated` fields
- ✅ **Maps** → `map<K,V>` (key must be string or integer)
- ✅ **Nested Structs** → nested messages
- ✅ **Optional Fields** → pointer types (`*T`)
- ✅ **Anonymous Structs** → auto-generated message names

#### Limitations on Complex Types
- ⚠️ **Slice of Pointers** (`[]*T`) - Not supported, use `[]T`
- ⚠️ **Map of Pointers** (`map[K]*V`) - Not supported, use `map[K]V`
- ✅ **Pointer to Struct** (`*Struct`) - Supported as optional field

### Protocol Support
- ✅ **gRPC** - Full protocol support with HTTP/2 (Protobuf only)
- ✅ **Connect Protocol** - Connect RPC protocol (Protobuf and JSON)
- ✅ **gRPC-Web** - Browser-friendly protocol with binary and base64 modes
- ✅ **JSON-RPC 2.0** - Single requests, batch requests, and notifications
- ✅ **REST/JSON** - Plain HTTP JSON endpoints (via Connect)
- ✅ **Protocol Auto-Detection** - Based on headers
- ✅ **Multi-Compression** - gzip, brotli, zstd support for all protocols

### Validation
- ✅ **Input Validation** - Using go-playground/validator
- ✅ **Validation Tags** - Standard validator tags work
- ✅ **Custom Validators** - Can register custom validation functions
- ✅ **Per-Method Control** - Enable/disable validation per method
- ✅ **Validation Metadata** - Tags are preserved in protobuf schema

### Service Discovery & Documentation
- ✅ **gRPC Reflection** - Full server reflection support
- ✅ **OpenAPI Generation** - Automatic OpenAPI 3.0 spec
- ✅ **grpcurl Compatible** - Works with standard gRPC tools
- ✅ **buf curl Compatible** - Works with buf tooling

### Performance Features
- ✅ **hyperpb Integration** - Faster dynamic protobuf parsing
- ✅ **Message Caching** - Schema and message type caching
- ✅ **PGO Support** - Profile-Guided Optimization for hyperpb
- ⚠️ **Message Pooling** - Limited due to hyperpb read-only constraint

### Developer Experience
- ✅ **No Proto Files** - Pure Go struct definitions
- ✅ **Type Safety** - Full Go type checking
- ✅ **JSON Tags** - Control field names via json tags
- ✅ **Fluent API** - Method chaining for configuration
- ✅ **Error Handling** - Structured errors with proper code mapping
- ✅ **Hot Reload** - Change handlers without restarting
- ✅ **Comment Preservation** - Go comments become proto documentation

## ❌ Not Supported Features

### Streaming
- ❌ **Server Streaming** - Not supported
- ❌ **Client Streaming** - Not supported
- ❌ **Bidirectional Streaming** - Not supported
- 💡 *Reason*: Current implementation focuses on unary RPCs

### Advanced Protobuf Features
- ✅ **Oneof Fields** - Supported via naming conventions and struct embedding
  - Automatic detection based on field naming patterns
  - Struct embedding with all pointer fields
  - Runtime validation enforces oneof constraints
- ✅ **Proto3 Optional** - Supported via pointer types
- ✅ **Protobuf Editions** - Edition 2023 supported
- ✅ **Enum Support** - Integer constants become enums
- ✅ **Well-Known Types** - Full support for:
  - ✅ `google.protobuf.Timestamp` - time.Time conversion
  - ✅ `google.protobuf.Duration` - time.Duration conversion
  - ✅ `google.protobuf.Empty` - struct{} or proto:"empty" tag  
  - ✅ `google.protobuf.Any` - dynamic message types (with caveats for JSON)
  - ✅ `google.protobuf.Struct` - dynamic JSON-like structures
  - ✅ `google.protobuf.Value` - any JSON value type
  - ✅ `google.protobuf.ListValue` - heterogeneous lists
  - ✅ `google.protobuf.FieldMask` - partial update support
- ❌ **Proto2 Syntax** - Only proto3/editions supported
- ❌ **Protobuf Extensions** - Not supported
- ❌ **Custom Options** - Limited support

### Other Limitations
- ❌ **Message Mutation** - hyperpb messages are read-only
- ❌ **Circular References** - Not supported in type definitions
- ❌ **Interface Types** - Cannot use interfaces in structs

### Interceptor Support
- ✅ **Built-in Interceptors** - Logging, Recovery, Timeout, Metrics
- ✅ **Custom Interceptors** - Full support for custom middleware
- ✅ **Service-level Interceptors** - Apply to all methods
- ✅ **Method-level Interceptors** - Apply to specific methods
- ✅ **Interceptor Chaining** - Multiple interceptors in order

### Proto Export
- ✅ **FileDescriptorSet Export** - Export complete schema
- ✅ **Proto File Generation** - Generate `.proto` files
- ✅ **CLI Tool** - Export from running service
- ✅ **Programmatic Export** - Export in code
- ✅ **Edition Support** - Export as proto3 or editions

## 🔧 Configuration Options

### Service Options
```go
rpc.NewService("ServiceName",
    rpc.WithPackage("package.v1"),             // ✅ Protobuf package
    rpc.WithValidation(true),                  // ✅ Enable validation
    rpc.WithReflection(true),                  // ✅ Enable reflection
    rpc.WithInterceptors(interceptor),         // ✅ Add interceptors
    rpc.WithEdition("2023"),                   // ✅ Use Protobuf Editions
    rpc.WithServiceConfig(jsonConfig),         // ✅ gRPC service config
    rpc.WithDescription("Service description"), // ✅ Documentation
)
```

### Method Options
```go
// Type-safe registration (recommended)
err := rpc.Register(svc, "MethodName", handler)
if err != nil {
    // handle error
}

// Builder pattern
rpc.MustRegisterMethod(svc, 
    rpc.NewMethod("MethodName", handler).
        In(RequestType{}).                         // ✅ Request type
        Out(ResponseType{}).                       // ✅ Response type
        Validate(true).                            // ✅ Override validation
        WithInterceptors(authInterceptor).         // ✅ Method interceptors
        WithDescription("Method description")      // ✅ Documentation
)
```

### Codec Options
```go
codec.DecoderOptions{
    EnablePooling: true,                // ⚠️ Limited effect
    AllowUnknownFields: false,          // ✅ Supported
    EnablePGO: true,                    // ✅ Supported
}
```

## 📊 Performance Characteristics

| Feature | Status | Impact |
|---------|--------|--------|
| Dynamic Schema Generation | ✅ | One-time cost at startup |
| Message Parsing (hyperpb) | ✅ | Significantly faster than dynamicpb |
| Message Encoding/Decoding | ✅ | Comparable to generated code |
| Memory Usage | ✅ | Similar to generated code (~10KB per request) |
| End-to-End Performance | ✅ | Within 5-10% of generated code |
| Allocations per Request | ⚠️ | ~30% more allocations than generated code |

## 🚀 Best Use Cases

Hyperway is ideal for:
- ✅ Rapid prototyping
- ✅ Internal services
- ✅ Services with simple request/response patterns
- ✅ Projects prioritizing development speed
- ✅ Services with frequently changing schemas

Not recommended for:
- ❌ Streaming-heavy applications (no streaming support)
- ❌ Services requiring proto2 features
- ⚠️ Extremely latency-sensitive services (5-10% overhead)
- ⚠️ Services with very high allocation pressure