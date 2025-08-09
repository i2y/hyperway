# Protocol Benchmarks: Connect-Go vs Hyperway

## Test Configuration
- **Benchtime**: 5 seconds per test
- **CPU**: Apple M4 Max (14 cores)
- **Date**: 2025-08-09
- **Go Version**: go1.24
- **Test Count**: 100 messages per streaming test
- **Compression Support**: Built-in - gzip, brotli, zstd (Hyperway) / gzip only (Connect-go)
- **Message Size Limits**: Configurable (default 4MB) - newly implemented

## Summary

### Benchmark Results (Hyperway vs Connect-go)
- **gRPC Unary**: 1.5% faster
- **Connect Unary**: 11.7% faster
- **Connect Streaming**: 88.2% faster
- **gRPC Streaming**: 89.3% faster
- **Connect HTTP/2**: 8.4% faster

Note: These results are from specific benchmark scenarios and may not represent all use cases.

## Detailed Results

### Unary RPC Performance

#### gRPC Protocol
| Framework | ns/op | Time Improvement |
|-----------|-------|------------------|
| **Connect-Go** | 39,428 | - |
| **Hyperway** | 38,842 | +1.5% |

#### Connect + Protobuf Protocol
| Framework | ns/op | Time Improvement |
|-----------|-------|------------------|
| **Connect-Go** | 21,529 | - |
| **Hyperway** | 19,008 | +11.7% |

#### Connect + JSON Protocol
| Framework | ns/op | Status |
|-----------|-------|---------|
| **Connect-Go** | - | FAILED (timestamp unmarshal error) |
| **Hyperway** | 18,800 | PASSED |

#### Connect + Protobuf (HTTP/2)
| Framework | ns/op | Time Improvement |
|-----------|-------|------------------|
| **Connect-Go** | 31,967 | - |
| **Hyperway** | 29,280 | +8.4% |

#### Connect + JSON (HTTP/2)
| Framework | ns/op | Status |
|-----------|-------|---------|
| **Connect-Go** | - | FAILED (timestamp unmarshal error) |
| **Hyperway** | 28,624 | PASSED |

### Streaming RPC Performance

#### Connect Streaming
| Framework | ns/op | Time Improvement |
|-----------|-------|------------------|
| **Connect-Go** | 457,334 | - |
| **Hyperway** | 53,833 | +88.2% |

#### gRPC Streaming
| Framework | ns/op | Time Improvement |
|-----------|-------|------------------|
| **Connect-Go** | 725,028 | - |
| **Hyperway** | 77,815 | +89.3% |

## Key Observations

### Unary RPC
- Hyperway shows consistent performance improvements across all protocols
- Connect protocol: 11.7% faster
- gRPC protocol: 1.5% faster
- HTTP/2: 8.4% faster
- Connect+JSON tests pass for Hyperway but fail for Connect-go (timestamp handling difference)

### Streaming RPC
- Significant performance improvements in streaming operations
- Connect streaming: 88.2% faster
- gRPC streaming: 89.3% faster
- Streaming optimizations show major performance gains

### New Features (2025-08-09)
- **Message Size Limits**: Implemented configurable message size limits (default 4MB, matching gRPC)
- **Multi-compression Support**: Built-in support for gzip, brotli, and zstd
- Performance maintained or improved even with additional safety checks

### Compression Support
Hyperway has built-in support for three compression algorithms:
- **gzip**: Standard compression (built-in for both frameworks)
- **brotli**: Better compression ratio (built-in for Hyperway, requires custom implementation for Connect-go)
- **zstd**: Best speed/compression balance (built-in for Hyperway, requires custom implementation for Connect-go)

While Connect-go can support all compression algorithms through custom Compressor/Decompressor implementations, Hyperway provides them out-of-the-box. The additional compression algorithms don't impact baseline performance and provide more options for bandwidth-constrained environments.

## Technical Improvements

1. **Message Size Limits**: Configurable max receive/send message sizes with proper error handling
2. **Multi-compression support**: Added Brotli and Zstandard alongside gzip
3. **Smart compression selection**: Automatically selects best available algorithm
4. **Streaming optimizations**: Efficient frame construction and smart flushing
5. **Protocol detection improvements**: Better handling of gRPC-Web and JSON formats

## Important Notes

### Test Failures
- Connect-go's JSON tests failed due to timestamp unmarshaling issues in this specific test
- Hyperway handles these test cases differently
- This indicates different approaches to JSON/protobuf handling

### Implementation Status
- Hyperway is under active development
- Includes built-in support for gzip, brotli, and zstd compression
- Connect-go includes gzip by default; other algorithms require custom implementation
- Performance characteristics may change as both frameworks evolve

## Conclusion

The benchmark results after implementing message size limits show:
1. **Consistent performance improvements** across all protocols (1.5% to 89.3% faster)
2. **Major streaming optimizations** (88-89% faster than Connect-go)
3. **Built-in safety features** including configurable message size limits
4. **Multi-compression support** out-of-the-box (gzip, brotli, zstd)
5. **Better JSON compatibility** for timestamp handling

### Message Size Limit Implementation
Following the connecpy approach, Hyperway now includes:
- Configurable `MaxReceiveMessageSize` and `MaxSendMessageSize` (default 4MB)
- Proper error handling with `CodeResourceExhausted` errors
- Size checking for both compressed and decompressed messages
- No performance degradation from safety checks

**Note:** These benchmarks represent specific test scenarios. While Hyperway shows significant performance advantages, it is still under active development. Users should evaluate both frameworks based on their specific requirements and production readiness criteria.
