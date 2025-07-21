package rpc

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const (
	testSuccess = "success"
)

func TestRetryPolicy(t *testing.T) {
	t.Run("Validation", func(t *testing.T) {
		// Valid policy
		valid := &RetryPolicy{
			MaxAttempts:          3,
			InitialBackoff:       "0.1s",
			MaxBackoff:           "10s",
			BackoffMultiplier:    2.0,
			RetryableStatusCodes: []string{"UNAVAILABLE"},
		}
		if err := ValidateRetryPolicy(valid); err != nil {
			t.Errorf("Valid policy failed validation: %v", err)
		}

		// Invalid max attempts
		invalid := &RetryPolicy{MaxAttempts: 1}
		if err := ValidateRetryPolicy(invalid); err == nil {
			t.Error("Expected error for maxAttempts <= 1")
		}

		// Invalid duration
		invalid = &RetryPolicy{
			MaxAttempts:    2,
			InitialBackoff: "invalid",
		}
		if err := ValidateRetryPolicy(invalid); err == nil {
			t.Error("Expected error for invalid duration")
		}

		// Invalid status code
		invalid = &RetryPolicy{
			MaxAttempts:          2,
			RetryableStatusCodes: []string{"INVALID_CODE"},
		}
		if err := ValidateRetryPolicy(invalid); err == nil {
			t.Error("Expected error for invalid status code")
		}
	})

	t.Run("Backoff Calculation", func(t *testing.T) {
		policy := &RetryPolicy{
			InitialBackoff:    "100ms",
			MaxBackoff:        "1s",
			BackoffMultiplier: 2.0,
		}

		// Test exponential backoff
		for i := 1; i <= 5; i++ {
			backoff := retryBackoff(policy, i)
			// Backoff should be approximately initialBackoff * (multiplier^(attempt-1))
			// With ±20% jitter
			expected := 100 * time.Millisecond * time.Duration(1<<(i-1))
			if expected > time.Second {
				expected = time.Second
			}

			// Check within reasonable bounds (accounting for jitter)
			minBackoff := time.Duration(float64(expected) * 0.7)
			maxBackoff := time.Duration(float64(expected) * 1.3)

			if backoff < minBackoff || backoff > maxBackoff {
				t.Errorf("Backoff for attempt %d = %v, expected between %v and %v",
					i, backoff, minBackoff, maxBackoff)
			}
		}
	})
}

func TestRetryInterceptor(t *testing.T) {
	// Create test service config
	config := &ServiceConfig{
		MethodConfig: []MethodConfig{
			{
				Name: []MethodName{
					{Service: "test.Service", Method: "TestMethod"},
				},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:          3,
					InitialBackoff:       "10ms",
					MaxBackoff:           "100ms",
					BackoffMultiplier:    2.0,
					RetryableStatusCodes: []string{"UNAVAILABLE"},
				},
			},
		},
	}

	interceptor := NewRetryInterceptor(config)

	t.Run("Successful Request", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			return testSuccess, nil
		}

		resp, err := interceptor.Intercept(context.Background(), "/test.Service/TestMethod", "req", handler)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp != testSuccess {
			t.Errorf("Expected 'success', got %v", resp)
		}
		if calls != 1 {
			t.Errorf("Expected 1 call, got %d", calls)
		}
	})

	t.Run("Retryable Error", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			if calls < 3 {
				return nil, &Error{
					Code:    CodeUnavailable,
					Message: "Service unavailable",
				}
			}
			return testSuccess, nil
		}

		start := time.Now()
		resp, err := interceptor.Intercept(context.Background(), "/test.Service/TestMethod", "req", handler)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp != testSuccess {
			t.Errorf("Expected 'success', got %v", resp)
		}
		if calls != 3 {
			t.Errorf("Expected 3 calls, got %d", calls)
		}

		// Check that backoff was applied (should take at least 10ms + 20ms)
		if duration < 25*time.Millisecond {
			t.Errorf("Expected backoff delay, but duration was only %v", duration)
		}
	})

	t.Run("Non-Retryable Error", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			return nil, &Error{
				Code:    CodeInvalidArgument,
				Message: "Invalid input",
			}
		}

		_, err := interceptor.Intercept(context.Background(), "/test.Service/TestMethod", "req", handler)
		if err == nil {
			t.Fatal("Expected error")
		}
		if calls != 1 {
			t.Errorf("Expected 1 call (no retry), got %d", calls)
		}
	})

	t.Run("Max Attempts Exceeded", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			return nil, &Error{
				Code:    CodeUnavailable,
				Message: "Always fails",
			}
		}

		_, err := interceptor.Intercept(context.Background(), "/test.Service/TestMethod", "req", handler)
		if err == nil {
			t.Fatal("Expected error")
		}
		if calls != 3 {
			t.Errorf("Expected 3 calls (max attempts), got %d", calls)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			// Always fail with retryable error
			return nil, &Error{
				Code:    CodeUnavailable,
				Message: fmt.Sprintf("Attempt %d fails", calls),
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		resp, err := interceptor.Intercept(ctx, "/test.Service/TestMethod", "req", handler)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context deadline exceeded, got %v (resp: %v, calls: %d)", err, resp, calls)
		}
		if calls > 2 {
			t.Errorf("Expected at most 2 calls before timeout, got %d", calls)
		}
	})
}

func TestRetryThrottling(t *testing.T) {
	config := &ServiceConfig{
		MethodConfig: []MethodConfig{
			{
				Name: []MethodName{{Service: "test.Service"}},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:          3,
					InitialBackoff:       "1ms",
					RetryableStatusCodes: []string{"UNAVAILABLE"},
				},
			},
		},
		RetryThrottling: &RetryThrottling{
			MaxTokens:  10,
			TokenRatio: 0.5,
		},
	}

	interceptor := NewRetryInterceptor(config)

	// Exhaust tokens with failures
	for i := 0; i < 15; i++ {
		handler := func(ctx context.Context, req any) (any, error) {
			return nil, &Error{Code: CodeUnavailable}
		}
		interceptor.Intercept(context.Background(), "/test.Service/Method", "req", handler)
	}

	// Now retries should be throttled
	calls := 0
	handler := func(ctx context.Context, req any) (any, error) {
		calls++
		return nil, &Error{Code: CodeUnavailable}
	}

	interceptor.Intercept(context.Background(), "/test.Service/Method", "req", handler)

	// Without throttling, we'd expect 3 calls (max attempts)
	// With throttling exhausted, we expect 1 call
	if calls != 1 {
		t.Errorf("Expected 1 call due to throttling, got %d", calls)
	}
}

func TestServerPushback(t *testing.T) {
	config := &ServiceConfig{
		MethodConfig: []MethodConfig{
			{
				Name: []MethodName{{Service: "test.Service"}},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:          3,
					RetryableStatusCodes: []string{"UNAVAILABLE"},
				},
			},
		},
	}

	interceptor := NewRetryInterceptor(config)

	t.Run("Positive Pushback", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			if calls == 1 {
				return nil, &Error{
					Code:    CodeUnavailable,
					Message: "Retry after delay",
					Details: map[string]any{
						"grpc-retry-pushback-ms": 50,
					},
				}
			}
			return testSuccess, nil
		}

		start := time.Now()
		resp, err := interceptor.Intercept(context.Background(), "/test.Service/Method", "req", handler)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp != testSuccess {
			t.Errorf("Expected success, got %v", resp)
		}
		if calls != 2 {
			t.Errorf("Expected 2 calls, got %d", calls)
		}
		// Should wait at least 50ms
		if duration < 45*time.Millisecond {
			t.Errorf("Expected pushback delay of ~50ms, but duration was %v", duration)
		}
	})

	t.Run("Negative Pushback (Don't Retry)", func(t *testing.T) {
		calls := 0
		handler := func(ctx context.Context, req any) (any, error) {
			calls++
			return nil, &Error{
				Code:    CodeUnavailable,
				Message: "Don't retry",
				Details: map[string]any{
					"grpc-retry-pushback-ms": -1,
				},
			}
		}

		_, err := interceptor.Intercept(context.Background(), "/test.Service/Method", "req", handler)
		if err == nil {
			t.Fatal("Expected error")
		}
		if calls != 1 {
			t.Errorf("Expected 1 call (no retry due to negative pushback), got %d", calls)
		}
	})
}

func TestParseServiceConfig(t *testing.T) {
	jsonConfig := `{
		"methodConfig": [{
			"name": [{"service": "test.Service", "method": "Method"}],
			"timeout": "5s",
			"retryPolicy": {
				"maxAttempts": 3,
				"initialBackoff": "0.1s",
				"maxBackoff": "10s",
				"backoffMultiplier": 2,
				"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
			}
		}],
		"retryThrottling": {
			"maxTokens": 100,
			"tokenRatio": 0.1
		}
	}`

	config, err := ParseServiceConfig(jsonConfig)
	if err != nil {
		t.Fatalf("Failed to parse service config: %v", err)
	}

	if len(config.MethodConfig) != 1 {
		t.Errorf("Expected 1 method config, got %d", len(config.MethodConfig))
	}

	mc := config.MethodConfig[0]
	if mc.Timeout != "5s" {
		t.Errorf("Expected timeout '5s', got %s", mc.Timeout)
	}

	if mc.RetryPolicy.MaxAttempts != 3 {
		t.Errorf("Expected max attempts 3, got %d", mc.RetryPolicy.MaxAttempts)
	}

	if config.RetryThrottling.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", config.RetryThrottling.MaxTokens)
	}
}
