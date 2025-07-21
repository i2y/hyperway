# Protobuf Editions Support

Hyperway supports both traditional proto3 syntax and the new Protobuf Editions syntax introduced in 2023. This document explains how to use Editions mode and the differences between the two approaches.

## What are Protobuf Editions?

Protobuf Editions is a new way to specify protobuf behavior using feature flags instead of syntax-specific rules. It provides:

- **Feature-based configuration** instead of syntax-specific behavior
- **Smooth evolution** of the protocol without breaking changes
- **Better defaults** based on lessons learned from proto2 and proto3
- **Future compatibility** for new features without syntax changes

## Using Editions in Hyperway

### Basic Usage

To create a service using Protobuf Editions:

```go
import (
    "github.com/i2y/hyperway/rpc"
    "github.com/i2y/hyperway/schema"
)

// Create a service with Editions 2023
svc := rpc.NewService("MyService",
    rpc.WithPackage("example.v1"),
    rpc.WithEdition(schema.Edition2023), // Enable Editions mode
)
```

### Supported Editions

- `schema.Edition2023` - The 2023 edition (currently supported)
- `schema.Edition2024` - The 2024 edition (may not be supported by all runtimes yet)

## Key Differences from Proto3

### 1. Syntax Declaration

**Proto3:**
```protobuf
syntax = "proto3";
```

**Editions:**
```protobuf
edition = "2023";
```

### 2. Field Presence

**Proto3:**
- Implicit presence by default
- Use `optional` keyword for explicit presence
- Pointer types in Go get `proto3_optional`

**Editions:**
- Controlled by `field_presence` feature
- Edition 2023 defaults to explicit presence
- No need for `proto3_optional` annotation

### 3. Feature Configuration

Editions use features to control behavior:

```go
// Custom features (advanced usage)
features := &schema.FeatureSet{
    FieldPresence:         schema.FieldPresenceExplicit,
    RepeatedFieldEncoding: schema.RepeatedFieldEncodingPacked,
    EnumType:              schema.EnumTypeOpen,
    UTF8Validation:        schema.UTF8ValidationVerify,
}

builder := schema.NewBuilder(schema.BuilderOptions{
    PackageName: "example.v1",
    SyntaxMode:  schema.SyntaxEditions,
    Edition:     schema.Edition2023,
    Features:    features,
})
```

## Example: Same Service in Both Modes

```go
// Message type used by both
type UserRequest struct {
    Name  string  `json:"name"`
    Email string  `json:"email"`
    Age   *int32  `json:"age,omitempty"` // Optional field
}

type UserResponse struct {
    ID      string `json:"id"`
    Message string `json:"message"`
}

// Handler function
func CreateUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
    return &UserResponse{
        ID:      "user-123",
        Message: fmt.Sprintf("Created user %s", req.Name),
    }, nil
}

// Proto3 version (default)
proto3Svc := rpc.NewService("UserService",
    rpc.WithPackage("example.v1"),
)

// Editions version
editionsSvc := rpc.NewService("UserService",
    rpc.WithPackage("example.v1"),
    rpc.WithEdition(schema.Edition2023),
)

// Register the same method on both
rpc.MustRegister(proto3Svc, rpc.NewMethod("CreateUser", CreateUser))
rpc.MustRegister(editionsSvc, rpc.NewMethod("CreateUser", CreateUser))
```

## Feature Defaults

### Proto3 Defaults
- Field Presence: Implicit
- Repeated Field Encoding: Packed
- Enum Type: Open
- UTF8 Validation: Verify

### Edition 2023 Defaults
- Field Presence: Explicit (like proto2 optional)
- Repeated Field Encoding: Packed
- Enum Type: Open
- UTF8 Validation: Verify

## Migration Guide

### From Proto3 to Editions

1. Add `rpc.WithEdition(schema.Edition2023)` to your service creation
2. Review optional field behavior - Editions default to explicit presence
3. Test your service - the wire format remains compatible

### Compatibility

- Services using different syntax modes can communicate with each other
- The wire format is the same - only the schema representation differs
- Editions are designed to be forward and backward compatible

## Best Practices

1. **Start with Editions for new projects** - It's the future of protobuf
2. **Use Edition 2023** - It's stable and well-supported
3. **Keep proto3 for existing services** - No need to migrate unless you need Editions features
4. **Test thoroughly when migrating** - Especially around optional field behavior

## Limitations

- Some protobuf runtimes may not support newer editions yet
- Edition 2024 support depends on the Go protobuf runtime version
- Some tools may not fully support Editions syntax yet

## Further Reading

- [Protobuf Editions Overview](https://protobuf.dev/editions/overview/)
- [Migrating to Protobuf Editions](https://protobuf.dev/editions/migration/)
- [Edition 2023 Features](https://protobuf.dev/editions/edition-2023/)