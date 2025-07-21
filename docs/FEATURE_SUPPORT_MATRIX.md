# Hyperway Feature Support Matrix

## ✅ Fully Supported Features

### Core RPC Functionality
- ✅ **Unary RPC** - Single request/response pattern
- ✅ **Dynamic Schema Generation** - Runtime protobuf schema from Go structs
- ✅ **Method Registration** - Fluent API for registering RPC methods
- ✅ **Service Grouping** - Multiple services in single gateway

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
- ✅ `time.Time` → `google.protobuf.Timestamp` (as JSON string)

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
- ✅ **gRPC** - Full protocol support with HTTP/2
- ✅ **Connect Protocol** - Connect RPC protocol
- ✅ **REST/JSON** - Plain HTTP JSON endpoints
- ✅ **Protocol Auto-Detection** - Based on headers

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
- ✅ **hyperpb Integration** - 10x faster dynamic protobuf parsing
- ✅ **Message Caching** - Schema and message type caching
- ✅ **PGO Support** - Profile-Guided Optimization for hyperpb
- ⚠️ **Message Pooling** - Limited due to hyperpb read-only constraint

### Developer Experience
- ✅ **No Proto Files** - Pure Go struct definitions
- ✅ **Type Safety** - Full Go type checking
- ✅ **JSON Tags** - Control field names via json tags
- ✅ **Fluent API** - Method chaining for configuration
- ✅ **Error Handling** - Automatic error mapping to protocols

## ❌ Not Supported Features

### Streaming
- ❌ **Server Streaming** - Not supported
- ❌ **Client Streaming** - Not supported
- ❌ **Bidirectional Streaming** - Not supported
- 💡 *Reason*: Architectural mismatch between dynamic types and Connect-go's streaming API

### Advanced Protobuf Features
- ✅ **Oneof Fields** - Supported via naming conventions and struct embedding
  - Automatic detection based on field naming patterns
  - Struct embedding with all pointer fields
  - Runtime validation enforces oneof constraints
- ❌ **Proto2 Syntax** - Only proto3 supported
- ❌ **Protobuf Extensions** - Not supported
- ❌ **Custom Options** - Limited support
- ❌ **Field Presence** - Proto3 default behavior only

### Other Limitations
- ❌ **gRPC-Web** - Requires additional proxy
- ❌ **Message Mutation** - hyperpb messages are read-only
- ❌ **Circular References** - Not supported in type definitions
- ❌ **Interface Types** - Cannot use interfaces in structs

## 🔧 Configuration Options

### Service Options
```go
rpc.NewService("ServiceName",
    rpc.WithPackage("package.v1"),      // ✅ Protobuf package
    rpc.WithValidation(true),           // ✅ Enable validation
    rpc.WithReflection(true),           // ✅ Enable reflection
)
```

### Method Options
```go
rpc.NewMethod("MethodName", handler).
    In(RequestType{}).                  // ✅ Request type
    Out(ResponseType{}).                // ✅ Response type
    Validate(true)                      // ✅ Override validation
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
| Message Parsing (hyperpb) | ✅ | 10x faster than dynamicpb |
| Message Encoding | ⚠️ | 2-3x slower than generated code |
| Memory Usage | ⚠️ | 1.5-2x more than generated code |
| End-to-End Performance | ⚠️ | 1.5-2x slower than generated code |

## 🚀 Best Use Cases

Hyperway is ideal for:
- ✅ Rapid prototyping
- ✅ Internal services
- ✅ Services with simple request/response patterns
- ✅ Projects prioritizing development speed
- ✅ Services with frequently changing schemas

Not recommended for:
- ❌ High-throughput production services
- ❌ Streaming-heavy applications
- ❌ Memory-constrained environments
- ❌ Services requiring proto2 features