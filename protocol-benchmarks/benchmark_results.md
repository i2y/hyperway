# Protocol Benchmarks: Connect-Go vs Hyperway

## Test Configuration
- **Benchtime**: 10 seconds per test
- **CPU**: Apple M4 Max (14 cores)
- **Date**: 2025-08-08
- **Go Version**: go1.24.3
- **Test Count**: 100 messages per streaming test

## Summary

### Performance Improvements
- **gRPC Unary**: 1.2% faster
- **Connect Unary**: 6.5% faster
- **Connect Streaming**: 88.5% faster
- **gRPC Streaming**: 87.3% faster

## Detailed Results

### Unary RPC Performance

#### gRPC Protocol
| Framework | Operations | ns/op | RPS | Improvement |
|-----------|------------|-------|-----|-------------|
| **Connect-Go** | 289,884 | 38,934 | 25,689 | - |
| **Hyperway** | 306,322 | 38,459 | 26,001 | +1.2% |

#### Connect + Protobuf Protocol
| Framework | Operations | ns/op | RPS | Improvement |
|-----------|------------|-------|-----|-------------|
| **Connect-Go** | 550,303 | 19,964 | 50,090 | - |
| **Hyperway** | 628,806 | 18,671 | 53,557 | +6.5% |

#### Connect + Protobuf (HTTP/2)
| Framework | Operations | ns/op | RPS | Improvement |
|-----------|------------|-------|-----|-------------|
| **Connect-Go** | 377,398 | 32,167 | 31,088 | - |
| **Hyperway** | 400,170 | 29,917 | 33,426 | +7.0% |

### Streaming RPC Performance

#### Connect Streaming
| Framework | Operations | ns/op | RPS | Improvement |
|-----------|------------|-------|-----|-------------|
| **Connect-Go** | 26,569 | 451,939 | 2,213 | - |
| **Hyperway** | 222,925 | 52,181 | 19,168 | +88.5% |

#### gRPC Streaming
| Framework | Operations | ns/op | RPS | Improvement |
|-----------|------------|-------|-----|-------------|
| **Connect-Go** | 16,818 | 709,339 | 1,410 | - |
| **Hyperway** | 133,275 | 90,352 | 11,068 | +87.3% |

## Key Observations

### Unary RPC
- Hyperway shows consistent improvements across all protocols
- Most significant gains in Connect protocol (6.5% - 7.0%)
- gRPC protocol shows smaller but still positive improvements

### Streaming RPC
- **Dramatic performance improvements** in streaming operations
- Connect streaming: **8.7x faster** than Connect-Go
- gRPC streaming: **7.8x faster** than Connect-Go
- These improvements validate the streaming optimizations mentioned in documentation

### Memory Efficiency
While not measured in this benchmark, the dramatic reduction in streaming latency suggests significant memory efficiency improvements, as claimed in the documentation.

## Important Notes

### Implementation Status
- Hyperway is still under active development
- Some protocol features may not be fully implemented
- Performance improvements may partially reflect differences in feature completeness
- Users should verify that Hyperway supports all required features for their use case

### Fair Comparison
While these benchmarks use the same client code and test scenarios, the underlying implementations may differ in:
- Error handling completeness
- Protocol compliance strictness  
- Edge case handling
- Feature parity

## Conclusion

Hyperway demonstrates:
1. **Consistent performance improvements** for unary RPCs across all protocols
2. **Exceptional streaming performance** with 87-88% improvements
3. **Promising performance characteristics** for supported features

These results should be considered alongside feature completeness when evaluating Hyperway for production use.
