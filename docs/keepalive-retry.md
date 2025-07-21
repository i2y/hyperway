# gRPC Keepalive and Retry Mechanisms

Hyperway implements gRPC-compliant keepalive and retry mechanisms according to the official gRPC specifications.

## Keepalive Support

Keepalive uses HTTP/2 PING frames to maintain connection health and prevent proxy timeouts.

### Client-Side Keepalive Parameters

```go
import "github.com/i2y/hyperway/gateway"

// Default keepalive (2 hours)
keepalive := gateway.DefaultKeepaliveParams()

// Aggressive keepalive for proxy environments
keepalive := gateway.AggressiveKeepaliveParams()

// Custom keepalive
keepalive := gateway.KeepaliveParameters{
    Time:                30 * time.Second,  // Send PING every 30s
    Timeout:             10 * time.Second,  // Timeout after 10s
    PermitWithoutStream: true,              // Allow PING without active calls
    MaxPingsWithoutData: 2,                 // Max pings without data
}
```

### Server-Side Keepalive Enforcement

```go
enforcement := gateway.KeepaliveEnforcementPolicy{
    MinTime:             5 * time.Minute,   // Min time between PINGs
    PermitWithoutStream: false,             // Require active streams
    MaxPingStrikes:      2,                 // Max bad pings before GOAWAY
}

// Configure gateway with keepalive
opts := gateway.Options{
    KeepaliveParams:            &keepalive,
    KeepaliveEnforcementPolicy: &enforcement,
}
```

### Keepalive Parameters Explained

| Parameter | Description | Default |
|-----------|-------------|---------|
| `Time` | Interval between keepalive pings | 2 hours |
| `Timeout` | Time to wait for ping acknowledgement | 20 seconds |
| `PermitWithoutStream` | Allow pings without active calls | false |
| `MaxPingsWithoutData` | Max pings when no data is being sent | 2 |
| `MinTime` | Minimum interval server accepts between pings | 5 minutes |
| `MaxPingStrikes` | Max "bad" pings before closing connection | 2 |

## Retry Support

Retry is configured via gRPC Service Config at per-method granularity.

### Basic Retry Configuration

```go
import "github.com/i2y/hyperway/rpc"

// Define retry policy
retryPolicy := &rpc.RetryPolicy{
    MaxAttempts:       4,
    InitialBackoff:    "0.1s",
    MaxBackoff:        "10s",
    BackoffMultiplier: 2.0,
    RetryableStatusCodes: []string{
        "UNAVAILABLE",
        "DEADLINE_EXCEEDED",
    },
}

// Create service config
serviceConfig := rpc.ServiceConfig{
    MethodConfig: []rpc.MethodConfig{
        {
            Name: []rpc.MethodName{
                {Service: "myapp.MyService", Method: "MyMethod"},
            },
            Timeout:     "30s",
            RetryPolicy: retryPolicy,
        },
    },
}

// Apply to service
svc := rpc.NewService("MyService",
    rpc.WithPackage("myapp"),
    rpc.WithServiceConfig(jsonConfig),
)
```

### Retry Policy Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| `MaxAttempts` | Maximum number of attempts (including original) | Yes, > 1 |
| `InitialBackoff` | Initial delay before first retry | Yes |
| `MaxBackoff` | Maximum delay between retries | Yes |
| `BackoffMultiplier` | Exponential backoff multiplier | Yes |
| `RetryableStatusCodes` | Which errors trigger retry | Yes |

### Retryable Status Codes

Common retryable codes:
- `UNAVAILABLE` - Service temporarily down
- `DEADLINE_EXCEEDED` - Request timeout
- `RESOURCE_EXHAUSTED` - Rate limiting
- `ABORTED` - Concurrency conflict

### Retry Throttling

Prevent retry storms with token bucket algorithm:

```go
RetryThrottling: &rpc.RetryThrottling{
    MaxTokens:  100,   // Max tokens in bucket (1-1000)
    TokenRatio: 0.1,   // Tokens added per successful RPC
}
```

### Using Retry Interceptor

```go
// Create retry interceptor
retryInterceptor := rpc.NewRetryInterceptor(&serviceConfig)

// Apply to specific methods
rpc.MustRegister(svc,
    rpc.NewMethod("MyMethod", handler).
        WithInterceptors(retryInterceptor),
)

// Or apply to all methods
svc := rpc.NewService("MyService",
    rpc.WithInterceptors(retryInterceptor),
)
```

### Server Pushback

Servers can control client retry behavior:

```go
// Tell client to retry after delay
return nil, &rpc.Error{
    Code: rpc.CodeUnavailable,
    Message: "Service busy",
    Details: map[string]interface{}{
        "grpc-retry-pushback-ms": 1000, // Retry after 1 second
    },
}

// Tell client not to retry
return nil, &rpc.Error{
    Code: rpc.CodeUnavailable,
    Message: "Do not retry",
    Details: map[string]interface{}{
        "grpc-retry-pushback-ms": -1, // Negative = don't retry
    },
}
```

## Complete Example

```go
package main

import (
    "github.com/i2y/hyperway/gateway"
    "github.com/i2y/hyperway/rpc"
)

func main() {
    // Configure retry
    serviceConfig := rpc.ServiceConfig{
        MethodConfig: []rpc.MethodConfig{{
            Name: []rpc.MethodName{{Service: "app.Service"}},
            RetryPolicy: rpc.DefaultRetryPolicy(),
        }},
        RetryThrottling: &rpc.RetryThrottling{
            MaxTokens: 100,
            TokenRatio: 0.1,
        },
    }

    // Create service with retry
    svc := rpc.NewService("Service",
        rpc.WithPackage("app"),
        rpc.WithServiceConfig(toJSON(serviceConfig)),
        rpc.WithInterceptors(
            rpc.NewRetryInterceptor(&serviceConfig),
        ),
    )

    // Configure keepalive
    gatewayOpts := gateway.Options{
        KeepaliveParams: &gateway.KeepaliveParameters{
            Time:                30 * time.Second,
            Timeout:             10 * time.Second,
            PermitWithoutStream: true,
        },
        KeepaliveEnforcementPolicy: &gateway.KeepaliveEnforcementPolicy{
            MinTime:        10 * time.Second,
            MaxPingStrikes: 5,
        },
    }

    // Create gateway
    gw, _ := gateway.New([]*gateway.Service{
        {Name: svc.Name(), Handlers: svc.Handlers()},
    }, gatewayOpts)

    // Start HTTP/2 server
    server := gateway.NewHTTP2Server(":8080", gw, gatewayOpts)
    server.ListenAndServe()
}
```

## Best Practices

### Keepalive
1. **Use aggressive keepalive** in environments with proxies that kill idle connections
2. **Coordinate client/server settings** to avoid "too_many_pings" errors
3. **Monitor ping strikes** to detect misconfigured clients

### Retry
1. **Be selective with retryable codes** - only retry truly transient failures
2. **Set reasonable max attempts** - typically 3-5 is sufficient
3. **Use exponential backoff** to avoid overwhelming servers
4. **Always configure retry throttling** to prevent retry storms
5. **Test retry behavior** under load to ensure stability

## Monitoring

Track these metrics:
- Keepalive ping failures
- Retry attempts and success rates
- Throttling token exhaustion
- Server pushback occurrences

## Limitations

1. HTTP/2 PING frame handling depends on the underlying HTTP/2 implementation
2. Retry only works for unary RPCs (not streaming)
3. Service config changes require service restart