# Hyperway Compatibility Test Suite

This test suite validates compatibility between Hyperway and Connect-go implementations.

## Overview

The compatibility tests ensure that Hyperway can serve as a drop-in replacement for Connect-go by testing:

- **Data Type Compatibility**: Basic scalars, nested messages, maps, repeated fields, oneof fields
- **Protocol Compatibility**: gRPC, Connect, and gRPC-Web protocols
- **Well-Known Types**: Timestamp, Duration, Wrappers, Empty
- **Streaming**: Server-streaming RPCs
- **Error Handling**: Proper error code propagation

## Test Structure

```
compatibility-tests/
├── service.go            # Service definitions and handlers
├── hyperway_server.go    # Hyperway server implementation
├── compatibility_test.go # Test cases
├── go.mod               # Module dependencies
└── README.md            # This file
```

## Running Tests

### Run all compatibility tests:
```bash
cd compatibility-tests
go test -v
```

### Run specific test:
```bash
go test -v -run TestSimpleDataTypes
go test -v -run TestComplexDataTypes
go test -v -run TestWellKnownTypes
go test -v -run TestServerStreaming
go test -v -run TestErrorHandling
```

### Generate proto files for Connect-go client:
```bash
go run export_proto.go
```

## Test Cases

### 1. Simple Data Types
Tests basic scalar type compatibility:
- String, int32, int64, float, double, bool, bytes
- Empty/zero values
- Negative numbers
- All protocols (Connect, gRPC, gRPC-Web)

### 2. Complex Data Types
Tests complex message structures:
- Nested messages
- Repeated fields (arrays)
- Map fields
- Optional fields
- Oneof fields

### 3. Well-Known Types
Tests Google's Well-Known Types:
- google.protobuf.Timestamp
- google.protobuf.Duration
- google.protobuf.StringValue
- google.protobuf.Int32Value
- google.protobuf.BoolValue
- google.protobuf.Empty

### 4. Server Streaming
Tests server-streaming RPC compatibility:
- Stream initialization
- Message delivery
- Stream completion

### 5. Error Handling
Tests error propagation:
- Success cases
- Various error codes
- Error message preservation

## Implementation Status

✅ **Completed:**
- Unary RPC compatibility
- All data type tests
- Protocol compatibility (Connect, gRPC, gRPC-Web)
- Server streaming tests
- Error handling tests

🚧 **In Progress:**
- Client streaming (when Hyperway supports it)
- Bidirectional streaming (when Hyperway supports it)

## Results

The test suite validates that Hyperway:
1. Correctly encodes/decodes all Protobuf types
2. Implements protocols compatible with Connect-go clients
3. Handles errors consistently with gRPC standards
4. Supports streaming operations

## Future Enhancements

- Add Connect-go server → Hyperway client tests
- Performance comparison benchmarks
- Load testing scenarios
- Metadata/header propagation tests
- Interceptor compatibility tests