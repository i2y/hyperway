# Protocol Benchmarks: Hyperway vs Connect-go

This directory contains comprehensive performance benchmarks comparing Hyperway with Connect-go, the reference implementation of the Connect protocol.

## Quick Results

Benchmark measurements show:
- Different performance characteristics for streaming operations
- Lower memory usage in tested scenarios
- Note: Hyperway is still in development and may not implement all Connect-go features

For detailed analysis, see [BENCHMARK_ANALYSIS.md](./BENCHMARK_ANALYSIS.md).

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

## Latest Benchmark Results

### Benchmark Measurements

| Protocol | Time Difference | Memory Difference |
|----------|------------------|------------------|
| **gRPC** | -2.9% | -37.9% |
| **Connect** | -10.5% | -43.1% |
| **Connect Streaming** | -90.3% | -95.1% |
| **gRPC Streaming** | -90.6% | -93.8% |
| **Connect HTTP/2** | -10.1% | -34.2% |

*Note: Negative values indicate Hyperway used less time/memory in these specific benchmarks. Results may vary based on use case and as Hyperway continues development.*

## Notes

The benchmarks show differences in performance characteristics between Hyperway and Connect-go. Important considerations:
- Hyperway is still under active development
- May not yet implement all features available in Connect-go
- Performance characteristics may change as development continues
- These benchmarks represent specific test scenarios

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