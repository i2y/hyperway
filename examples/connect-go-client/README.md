# Connect-go Client with Hyperway Server (Real Implementation)

This example demonstrates how to use **actual Connect-go generated clients** with a Hyperway server. Unlike the basic example that uses plain HTTP, this uses the real Connect-go workflow with `.proto` files and code generation.

## 🎯 Key Achievement

**This proves that Hyperway is 100% compatible with Connect-go clients!** You can:
1. Export `.proto` files from Hyperway's dynamic schemas
2. Generate standard Connect-go client code
3. Use the generated client to communicate with Hyperway servers

## Overview

This example shows the complete workflow:
1. **Hyperway Server**: Defines services using Go structs (no `.proto` files needed)
2. **Export Proto**: Hyperway exports `.proto` files from the running service
3. **Code Generation**: Use `buf` to generate Connect-go client code
4. **Connect Client**: Use the generated client to communicate with Hyperway

## Features Demonstrated

- ✅ Full CRUD operations (Create, Read, Update, List)
- ✅ Proto file export from Hyperway
- ✅ Standard Connect-go code generation
- ✅ Type-safe client-server communication
- ✅ Custom headers support
- ✅ All Connect protocols (gRPC, Connect, gRPC-Web)

## Project Structure

```
proper/
├── server/
│   └── main.go           # Hyperway server implementation
├── client/
│   └── main.go           # Connect-go client using generated code
├── gen/                  # Generated code directory
│   ├── user.v1.pb.go    # Protobuf message definitions
│   └── userv1connect/    
│       └── user.v1.connect.go  # Connect client/server code
├── user.v1.proto        # Exported proto file
├── buf.yaml             # Buf configuration
└── buf.gen.yaml         # Code generation configuration
```

## Running the Example

### 1. Export Proto File

First, export the `.proto` file from the Hyperway service:

```bash
go run server/main.go export-proto
```

This creates:
- `user.v1.proto` - The service definition with `go_package` option automatically added
- `google/protobuf/timestamp.proto` - Well-known types

**Note**: The `go_package` option is now automatically added during export, eliminating the need for manual editing!

### 2. Generate Connect-go Code

```bash
# Install required tools (if not already installed)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

# Generate code using buf
buf generate
```

This generates:
- `gen/user.v1.pb.go` - Protobuf Go types
- `gen/userv1connect/user.v1.connect.go` - Connect client/server interfaces

### 3. Start the Server

```bash
go run server/main.go
```

The server runs on port 8888 and supports:
- gRPC protocol
- Connect protocol  
- gRPC-Web protocol

### 4. Run the Client

```bash
go run client/main.go -server=http://localhost:8888
```

## The Workflow Explained

### Step 1: Define Service in Go (Hyperway)

```go
// No .proto file needed!
type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}
```

### Step 2: Export Proto

Hyperway dynamically generates the `.proto` file:

```proto
service UserService {
  rpc CreateUser (CreateUserRequest) returns (CreateUserResponse);
  rpc GetUser (GetUserRequest) returns (GetUserResponse);
  rpc ListUsers (ListUsersRequest) returns (ListUsersResponse);
}
```

### Step 3: Generate Connect Client

Standard Connect-go code generation:

```go
// Generated client interface
type UserServiceClient interface {
    CreateUser(context.Context, *connect.Request[gen.CreateUserRequest]) 
        (*connect.Response[gen.CreateUserResponse], error)
    // ...
}
```

### Step 4: Use the Client

```go
// Real Connect-go client usage
client := userv1connect.NewUserServiceClient(
    http.DefaultClient,
    "http://localhost:8888",
)

req := connect.NewRequest(&userv1.CreateUserRequest{
    Name:  "Alice",
    Email: "alice@example.com",
})

resp, err := client.CreateUser(ctx, req)
```

## Why This Matters

1. **Zero Lock-in**: Start with Hyperway, export to standard `.proto` anytime
2. **Full Compatibility**: Works with any Connect/gRPC tooling
3. **Best of Both Worlds**: 
   - Development speed of code-first approach
   - Interoperability of schema-first approach
4. **Migration Path**: Easy to migrate between Hyperway and traditional protoc workflow

## Testing Different Protocols

### Connect Protocol (JSON)
```bash
curl -X POST http://localhost:8888/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name":"Test","email":"test@example.com"}'
```

### gRPC (with grpcurl)
```bash
# List services
grpcurl -plaintext localhost:8888 list

# Call method
grpcurl -plaintext -d '{"name":"Test","email":"test@example.com"}' \
  localhost:8888 user.v1.UserService/CreateUser
```

## Performance Notes

The generated Connect-go client communicates with Hyperway server with:
- Native protocol support (no translation layer)
- Full type safety
- Minimal overhead
- Standard Connect interceptors support

## Conclusion

This example proves that Hyperway is a drop-in replacement for traditional protoc-based workflows while offering the flexibility of code-first development. Teams can:

1. Start development immediately without writing `.proto` files
2. Export `.proto` files when needed for cross-team collaboration
3. Use standard Connect/gRPC tooling and clients
4. Maintain full compatibility with the broader gRPC ecosystem

**Hyperway + Connect-go = Best of both worlds! 🚀**