# gRPC Protocol Benchmark: Connect-Go vs Hyperway

## Test Configuration
- **Protocol**: Real gRPC with Protobuf binary encoding
- **Keep-alive**: Enabled on both servers
- **HTTP/2**: Enabled without TLS (h2c)
- **Benchtime**: 10 seconds
- **CPU**: Apple M4 Max (14 cores)
- **Date**: 2025-07-26

## Results

### gRPC Protocol
| Framework | Operations | ns/op | RPS |
|-----------|------------|-------|-----|
| **Connect-Go** | 301,876 | 39,440 | 25,355 |
| **Hyperway** | 299,166 | 39,690 | 25,199 |

### Connect + Protobuf Protocol
| Framework | Operations | ns/op | RPS |
|-----------|------------|-------|-----|
| **Connect-Go** | 664,591 | 16,588 | 60,284 |
| **Hyperway** | 763,724 | 14,919 | 67,031 |

### Connect + JSON Protocol (Apache Bench)
| Framework | RPS | Mean Latency |
|-----------|-----|--------------|
| **Connect-Go** | 127,145 | 0.787 ms |
| **Hyperway** | 134,987 | 0.741 ms |

## Performance Summary

### gRPC Protocol
- **Connect-Go**: 25,355 RPS (39.4 μs/req)
- **Hyperway**: 25,199 RPS (39.7 μs/req)
- **Difference**: Nearly identical performance (~0.6% difference)

### Connect + Protobuf Protocol
- **Connect-Go**: 60,284 RPS (16.6 μs/req)
- **Hyperway**: 67,031 RPS (14.9 μs/req)
- **Difference**: Hyperway is 11.2% faster

### Connect + JSON Protocol
- **Connect-Go**: 127,145 RPS (0.787 ms/req)
- **Hyperway**: 134,987 RPS (0.741 ms/req)
- **Difference**: Hyperway is 6.2% faster
