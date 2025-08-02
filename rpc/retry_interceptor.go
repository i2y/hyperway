// Package rpc provides retry interceptor implementation.
package rpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Constants
const (
	methodPartsCount          = 3
	throttleInitialTokenRatio = 2
)

// RetryInterceptor implements retry logic according to gRPC specification.
type RetryInterceptor struct {
	serviceConfig *ServiceConfig
	throttle      *retryThrottle
}

// retryThrottle implements token bucket algorithm for retry throttling.
type retryThrottle struct {
	mu         sync.Mutex
	maxTokens  float64
	tokens     float64
	tokenRatio float64
}

// NewRetryInterceptor creates a new retry interceptor with the given service config.
func NewRetryInterceptor(config *ServiceConfig) *RetryInterceptor {
	interceptor := &RetryInterceptor{
		serviceConfig: config,
	}

	// Initialize throttle if configured
	if config != nil && config.RetryThrottling != nil {
		interceptor.throttle = &retryThrottle{
			maxTokens:  float64(config.RetryThrottling.MaxTokens),
			tokens:     float64(config.RetryThrottling.MaxTokens) / throttleInitialTokenRatio, // Start at half capacity
			tokenRatio: config.RetryThrottling.TokenRatio,
		}
	}

	return interceptor
}

// Intercept implements the Interceptor interface with retry logic.
func (r *RetryInterceptor) Intercept(
	ctx context.Context,
	method string,
	req any,
	handler func(context.Context, any) (any, error),
) (any, error) {
	// Find retry policy for this method
	policy := r.findRetryPolicy(method)
	if policy == nil {
		// No retry policy, execute once
		return handler(ctx, req)
	}

	// Check if we have tokens for retry
	if !r.checkThrottle() {
		// No tokens available, execute once without retry
		return handler(ctx, req)
	}

	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Execute the request
		resp, err := handler(ctx, req)

		if err == nil {
			// Success! Add tokens back
			r.addTokens()
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err, policy) {
			return nil, err
		}

		// Check if this is the last attempt
		if attempt >= policy.MaxAttempts {
			break
		}

		// Check for server pushback
		if pushbackMs := extractPushbackMs(err); pushbackMs != 0 {
			if pushbackMs < 0 {
				// Negative value means don't retry
				return nil, err
			}
			// Wait for pushback duration
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(pushbackMs) * time.Millisecond):
			}
			continue
		}

		// Calculate backoff
		backoff := retryBackoff(policy, attempt)

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		// Consume a token for retry
		if !r.consumeToken() {
			// No more tokens, stop retrying
			break
		}
	}

	return nil, lastErr
}

// findRetryPolicy finds the retry policy for a given method.
func (r *RetryInterceptor) findRetryPolicy(method string) *RetryPolicy {
	if r.serviceConfig == nil {
		return nil
	}

	// Method format: /package.Service/Method
	parts := strings.Split(method, "/")
	if len(parts) != methodPartsCount {
		return nil
	}

	serviceName := parts[1]
	methodName := parts[2]

	// Find matching method config
	for _, mc := range r.serviceConfig.MethodConfig {
		for _, name := range mc.Name {
			// Check if service matches
			if name.Service != serviceName {
				continue
			}

			// If method is empty, it applies to all methods in the service
			if name.Method == "" || name.Method == methodName {
				return mc.RetryPolicy
			}
		}
	}

	return nil
}

// isRetryable checks if an error is retryable according to the policy.
func (r *RetryInterceptor) isRetryable(err error, policy *RetryPolicy) bool {
	if err == nil || policy == nil {
		return false
	}

	// Extract status code from error
	code := extractStatusCode(err)

	// Check if code is in retryable list
	for _, retryableCode := range policy.RetryableStatusCodes {
		if code == retryableCode {
			return true
		}
	}

	return false
}

// checkThrottle checks if retry is allowed by throttle.
func (r *RetryInterceptor) checkThrottle() bool {
	if r.throttle == nil {
		return true
	}

	r.throttle.mu.Lock()
	defer r.throttle.mu.Unlock()

	return r.throttle.tokens >= 1
}

// consumeToken consumes a token for retry.
func (r *RetryInterceptor) consumeToken() bool {
	if r.throttle == nil {
		return true
	}

	r.throttle.mu.Lock()
	defer r.throttle.mu.Unlock()

	if r.throttle.tokens >= 1 {
		r.throttle.tokens--
		return true
	}

	return false
}

// addTokens adds tokens back after successful RPC.
func (r *RetryInterceptor) addTokens() {
	if r.throttle == nil {
		return
	}

	r.throttle.mu.Lock()
	defer r.throttle.mu.Unlock()

	// Add tokens based on token ratio
	r.throttle.tokens += r.throttle.tokenRatio

	// Cap at max tokens
	if r.throttle.tokens > r.throttle.maxTokens {
		r.throttle.tokens = r.throttle.maxTokens
	}
}

// Status code constants for retry
const (
	statusUnknown = "UNKNOWN"
)

// Map of RPC codes to gRPC status codes
var codeToStatusMap = map[Code]string{
	CodeCanceled:           "CANCELLED",
	CodeUnknown:            statusUnknown,
	CodeInvalidArgument:    "INVALID_ARGUMENT",
	CodeDeadlineExceeded:   "DEADLINE_EXCEEDED",
	CodeNotFound:           "NOT_FOUND",
	CodeAlreadyExists:      "ALREADY_EXISTS",
	CodePermissionDenied:   "PERMISSION_DENIED",
	CodeResourceExhausted:  "RESOURCE_EXHAUSTED",
	CodeFailedPrecondition: "FAILED_PRECONDITION",
	CodeAborted:            "ABORTED",
	CodeOutOfRange:         "OUT_OF_RANGE",
	CodeUnimplemented:      "UNIMPLEMENTED",
	CodeInternal:           "INTERNAL",
	CodeUnavailable:        "UNAVAILABLE",
	CodeDataLoss:           "DATA_LOSS",
	CodeUnauthenticated:    "UNAUTHENTICATED",
}

// extractStatusCode extracts the gRPC status code from an error.
func extractStatusCode(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's an RPC error with a code
	if rpcErr, ok := err.(*Error); ok {
		if status, ok := codeToStatusMap[rpcErr.Code]; ok {
			return status
		}
		return statusUnknown
	}

	// Try to extract from error message
	errStr := err.Error()

	// Look for common patterns
	statusCodes := []string{
		"CANCELLED",
		"UNKNOWN",
		"INVALID_ARGUMENT",
		"DEADLINE_EXCEEDED",
		"NOT_FOUND",
		"ALREADY_EXISTS",
		"PERMISSION_DENIED",
		"RESOURCE_EXHAUSTED",
		"FAILED_PRECONDITION",
		"ABORTED",
		"OUT_OF_RANGE",
		"UNIMPLEMENTED",
		"INTERNAL",
		"UNAVAILABLE",
		"DATA_LOSS",
		"UNAUTHENTICATED",
	}

	for _, code := range statusCodes {
		if strings.Contains(strings.ToUpper(errStr), code) {
			return code
		}
	}

	return "UNKNOWN"
}

// extractPushbackMs extracts grpc-retry-pushback-ms from error metadata.
func extractPushbackMs(err error) int {
	// Check if error has metadata
	if rpcErr, ok := err.(*Error); ok {
		if rpcErr.Details != nil {
			// Look for pushback in details
			if pushback, ok := rpcErr.Details["grpc-retry-pushback-ms"]; ok {
				if ms, ok := pushback.(int); ok {
					return ms
				}
				if msStr, ok := pushback.(string); ok {
					var ms int
					if _, err := fmt.Sscanf(msStr, "%d", &ms); err == nil {
						return ms
					}
				}
			}
		}
	}

	return 0
}

// Default retry throttling constants
const (
	defaultMaxTokens  = 100
	defaultTokenRatio = 0.1
)

// DefaultRetryInterceptor creates a retry interceptor with default retry policy.
func DefaultRetryInterceptor() *RetryInterceptor {
	config := &ServiceConfig{
		MethodConfig: []MethodConfig{
			{
				Name: []MethodName{
					{Service: ""}, // Apply to all services
				},
				RetryPolicy: DefaultRetryPolicy(),
			},
		},
		RetryThrottling: &RetryThrottling{
			MaxTokens:  defaultMaxTokens,
			TokenRatio: defaultTokenRatio,
		},
	}

	return NewRetryInterceptor(config)
}
