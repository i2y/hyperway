// Package rpc provides retry policy support according to gRPC specification.
package rpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"time"
)

// RetryPolicy defines the retry configuration for a method according to gRPC spec.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts including the original request.
	// Must be greater than 1. Required.
	MaxAttempts int `json:"maxAttempts"`

	// InitialBackoff is the initial delay before the first retry.
	// Format: "0.1s", "100ms", etc.
	InitialBackoff string `json:"initialBackoff"`

	// MaxBackoff is the maximum delay between retries.
	// Format: "30s", "1m", etc.
	MaxBackoff string `json:"maxBackoff"`

	// BackoffMultiplier is the multiplier for exponential backoff.
	// Default: 2.0
	BackoffMultiplier float64 `json:"backoffMultiplier"`

	// RetryableStatusCodes defines which status codes should trigger a retry.
	// Common values: ["UNAVAILABLE"], ["DEADLINE_EXCEEDED"], ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
	RetryableStatusCodes []string `json:"retryableStatusCodes"`
}

// RetryThrottling controls client-side retry throttling.
type RetryThrottling struct {
	// MaxTokens is the maximum number of tokens in the bucket.
	// Must be in range (0, 1000]. Required.
	MaxTokens int `json:"maxTokens"`

	// TokenRatio is the ratio of tokens to add on each successful RPC.
	// Must be greater than 0. Decimal places beyond 3 are ignored.
	TokenRatio float64 `json:"tokenRatio"`
}

// MethodConfig defines the configuration for specific methods.
type MethodConfig struct {
	// Name identifies the methods to which this configuration applies.
	Name []MethodName `json:"name"`

	// Timeout for the method. Format: "1s", "100ms", etc.
	Timeout string `json:"timeout,omitempty"`

	// RetryPolicy for the method.
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

// MethodName identifies a gRPC method.
type MethodName struct {
	// Service name including proto package name. Required.
	Service string `json:"service"`

	// Method name. If empty, applies to all methods in the service.
	Method string `json:"method,omitempty"`
}

// ServiceConfig represents the complete service configuration.
type ServiceConfig struct {
	// MethodConfig contains per-method configuration.
	MethodConfig []MethodConfig `json:"methodConfig,omitempty"`

	// RetryThrottling controls client-side retry throttling.
	RetryThrottling *RetryThrottling `json:"retryThrottling,omitempty"`
}

// ValidateRetryPolicy validates a retry policy according to gRPC spec.
func ValidateRetryPolicy(policy *RetryPolicy) error {
	if policy == nil {
		return nil
	}

	// maxAttempts MUST be specified and MUST be greater than 1
	if policy.MaxAttempts <= 1 {
		return fmt.Errorf("maxAttempts must be greater than 1, got %d", policy.MaxAttempts)
	}

	// Parse and validate durations
	if policy.InitialBackoff != "" {
		if _, err := time.ParseDuration(policy.InitialBackoff); err != nil {
			return fmt.Errorf("invalid initialBackoff: %w", err)
		}
	}

	if policy.MaxBackoff != "" {
		if _, err := time.ParseDuration(policy.MaxBackoff); err != nil {
			return fmt.Errorf("invalid maxBackoff: %w", err)
		}
	}

	// Validate backoff multiplier
	if policy.BackoffMultiplier < 0 {
		return fmt.Errorf("backoffMultiplier must be non-negative, got %f", policy.BackoffMultiplier)
	}

	// Validate status codes
	validStatusCodes := map[string]bool{
		"CANCELLED":           true, //nolint:misspell // gRPC uses British spelling
		"UNKNOWN":             true,
		"INVALID_ARGUMENT":    true,
		"DEADLINE_EXCEEDED":   true,
		"NOT_FOUND":           true,
		"ALREADY_EXISTS":      true,
		"PERMISSION_DENIED":   true,
		"RESOURCE_EXHAUSTED":  true,
		"FAILED_PRECONDITION": true,
		"ABORTED":             true,
		"OUT_OF_RANGE":        true,
		"UNIMPLEMENTED":       true,
		"INTERNAL":            true,
		"UNAVAILABLE":         true,
		"DATA_LOSS":           true,
		"UNAUTHENTICATED":     true,
	}

	for _, code := range policy.RetryableStatusCodes {
		if !validStatusCodes[code] {
			return fmt.Errorf("invalid retryable status code: %s", code)
		}
	}

	return nil
}

// ValidateRetryThrottling validates retry throttling configuration.
func ValidateRetryThrottling(throttling *RetryThrottling) error {
	if throttling == nil {
		return nil
	}

	// maxTokens MUST be in range (0, 1000]
	if throttling.MaxTokens <= 0 || throttling.MaxTokens > 1000 {
		return fmt.Errorf("maxTokens must be in range (0, 1000], got %d", throttling.MaxTokens)
	}

	// tokenRatio MUST be greater than 0
	if throttling.TokenRatio <= 0 {
		return fmt.Errorf("tokenRatio must be greater than 0, got %f", throttling.TokenRatio)
	}

	return nil
}

// ParseServiceConfig parses a JSON service configuration.
func ParseServiceConfig(jsonConfig string) (*ServiceConfig, error) {
	var config ServiceConfig
	if err := json.Unmarshal([]byte(jsonConfig), &config); err != nil {
		return nil, fmt.Errorf("failed to parse service config: %w", err)
	}

	// Validate all retry policies
	for i, mc := range config.MethodConfig {
		if err := ValidateRetryPolicy(mc.RetryPolicy); err != nil {
			return nil, fmt.Errorf("invalid retry policy in methodConfig[%d]: %w", i, err)
		}
	}

	// Validate retry throttling
	if err := ValidateRetryThrottling(config.RetryThrottling); err != nil {
		return nil, fmt.Errorf("invalid retry throttling: %w", err)
	}

	return &config, nil
}

// retryBackoff calculates the backoff duration for a retry attempt.
func retryBackoff(policy *RetryPolicy, attempt int) time.Duration {
	if policy == nil || attempt <= 0 {
		return 0
	}

	// Parse initial backoff
	initialBackoff, err := time.ParseDuration(policy.InitialBackoff)
	if err != nil {
		initialBackoff = 100 * time.Millisecond // Default
	}

	// Parse max backoff
	maxBackoff, err := time.ParseDuration(policy.MaxBackoff)
	if err != nil {
		maxBackoff = 30 * time.Second // Default
	}

	// Calculate exponential backoff
	multiplier := policy.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2.0 // Default
	}

	backoff := float64(initialBackoff) * math.Pow(multiplier, float64(attempt-1))

	// Cap at max backoff
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// Apply jitter of ±20%
	jitter := 0.2
	jitterRange := backoff * jitter

	// Generate cryptographically secure random number for jitter
	// We need a random value in range [-jitterRange, +jitterRange]
	// First get a random value in range [0, 2*jitterRange]
	maxJitter := int64(2 * jitterRange)
	if maxJitter <= 0 {
		return time.Duration(backoff)
	}

	randomBigInt, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
	if err != nil {
		// Fallback to no jitter on error
		return time.Duration(backoff)
	}

	// Convert to [-jitterRange, +jitterRange] by subtracting jitterRange
	randomJitter := float64(randomBigInt.Int64()) - jitterRange
	actualBackoff := backoff + randomJitter

	return time.Duration(actualBackoff)
}

// Default retry configuration constants
const (
	defaultMaxAttempts          = 3
	defaultBackoffMultiplier    = 2.0
	aggressiveMaxAttempts       = 5
	aggressiveBackoffMultiplier = 1.5
)

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       defaultMaxAttempts,
		InitialBackoff:    "0.1s",
		MaxBackoff:        "10s",
		BackoffMultiplier: defaultBackoffMultiplier,
		RetryableStatusCodes: []string{
			"UNAVAILABLE",
			"DEADLINE_EXCEEDED",
		},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations.
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       aggressiveMaxAttempts,
		InitialBackoff:    "0.05s",
		MaxBackoff:        "30s",
		BackoffMultiplier: aggressiveBackoffMultiplier,
		RetryableStatusCodes: []string{
			"UNAVAILABLE",
			"DEADLINE_EXCEEDED",
			"RESOURCE_EXHAUSTED",
			"ABORTED",
		},
	}
}
