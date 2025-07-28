# gRPC Protocol Performance Comparison

This directory contains performance benchmarks comparing Hyperway with Connect-Go across different protocols.

For detailed benchmark results, see [benchmark_results.md](benchmark_results.md).

## Setup

1. Install dependencies:
```bash
go mod download
```

2. Generate protobuf code:
```bash
buf generate
```

3. Start servers:
```bash
# Terminal 1: Hyperway server (port 8080)
cd ../examples/grpcweb
go run .

# Terminal 2: Connect-Go server (port 8084)
go run connect_server.go
```

## Running Benchmarks

### Go Benchmarks (gRPC and Connect+Protobuf)
```bash
# All protocols
go test -bench=. -benchtime=30s

# gRPC protocol only
go test -bench='GRPC' -benchtime=30s

# Connect protocol only  
go test -bench='Connect' -benchtime=30s
```

### Apache Bench (Connect+JSON)
```bash
# Run the automated script
./run_apache_bench.sh

# Or run manually:
# Connect-Go
ab -n 100000 -c 100 -k -p /dev/stdin -T "application/json" \
   http://127.0.0.1:8084/grpcweb.example.v1.GreeterService/Greet <<< '{"name":"Test"}'

# Hyperway
ab -n 100000 -c 100 -k -p /dev/stdin -T "application/json" \
   http://127.0.0.1:8080/grpcweb.example.v1.GreeterService/Greet <<< '{"name":"Test"}'
```

## Latest Results

### Unary RPC Performance

Hyperway shows competitive performance compared to connect-go across all protocols for unary RPCs.

### Streaming RPC Performance

Hyperway demonstrates significant improvements in streaming performance, with substantially reduced memory usage and lower latency compared to connect-go.

## Summary

Hyperway provides comparable or better performance across all tested protocols, with particular strengths in:
- Memory efficiency
- Streaming performance
- Protocol compatibility

## Key Improvements Applied

1. **Eliminated JSON conversion in gRPC encoding** - Direct struct to protobuf conversion
2. **Enabled PGO (Profile-Guided Optimization)** for hyperpb
3. **Optimized memory allocations** - Reduced intermediate objects
4. **Added connection pooling** for HTTP/1.1 benchmarks
5. **Smart flushing for streaming** - 10ms intervals instead of per-message
6. **Lock-free message encoding** - Minimal critical sections
7. **Buffer pooling** - Reuse frame buffers with sync.Pool
8. **Cached encoding decisions** - Pre-determine encoder based on protocol

## Protocol Testing with buf curl

### gRPC
```bash
buf curl --protocol grpc --schema greeter.proto \
  --data '{"name":"Test"}' --http2-prior-knowledge \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet
```

### Connect + Protobuf
```bash
buf curl --protocol connect --schema greeter.proto \
  --data '{"name":"Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet
```

### gRPC-Web
```bash
buf curl --protocol grpcweb --schema greeter.proto \
  --data '{"name":"Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet
```