# Proto Export

This package provides functionality to export dynamically generated Protobuf schemas as `.proto` files.

## Features

- Export FileDescriptorSets to `.proto` files
- Support for multiple files with proper imports
- ZIP export for easy distribution
- HTTP endpoints for serving proto files
- CLI tool for command-line export

## Usage

### Programmatic API

```go
// Export service definition only
protoContent, err := svc.ExportProto()

// Export all proto files (service + messages)
allProtos, err := svc.ExportAllProtos()

// Get raw FileDescriptorSet
fdset := svc.GetFileDescriptorSet()
```

### HTTP Endpoints

When using the gateway, proto files are automatically served:

```bash
# List available proto files
curl http://localhost:8080/proto

# Download specific proto file
curl http://localhost:8080/proto/user.v1.proto

# Download all as ZIP
curl -O http://localhost:8080/proto.zip
```

### CLI Tool

```bash
# Export to stdout
go run examples/export-proto/main.go

# Export to directory
go run examples/export-proto/main.go -output ./proto

# Export as single file
go run examples/export-proto/main.go -single -output ./proto

# Export as ZIP
go run examples/export-proto/main.go -zip proto.zip
```

## Implementation Details

The proto export functionality uses the [jhump/protoreflect](https://github.com/jhump/protoreflect) library to convert FileDescriptorSets back to `.proto` source files. The exported files maintain proper:

- Package declarations
- Import statements
- Message definitions with field types and tags
- Service definitions with RPC methods
- Validation metadata in field options

## Example Output

```proto
syntax = "proto3";

package user.v1;

import "user.v1/createuserrequest.proto";
import "user.v1/createuserresponse.proto";

service UserService {
  rpc CreateUser ( CreateUserRequest ) returns ( CreateUserResponse );
  rpc GetUser ( GetUserRequest ) returns ( GetUserResponse );
}
```

## Benefits

1. **Interoperability**: Generate proto files for use with other gRPC tools
2. **Documentation**: Human-readable API definitions
3. **Code Generation**: Use exported protos with standard protoc toolchain
4. **Version Control**: Track API changes in proto format
5. **Client SDKs**: Generate clients in any language from exported protos