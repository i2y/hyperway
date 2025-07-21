package rpc

import (
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

const testResponse = "response"

func TestInterceptors(t *testing.T) {
	t.Run("LoggingInterceptor", func(t *testing.T) {
		var logs []string
		logger := log.New(&testWriter{logs: &logs}, "", 0)

		interceptor := &LoggingInterceptor{Logger: logger}

		// Test successful request
		resp, err := interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			return testResponse, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp != testResponse {
			t.Errorf("Expected '%s', got %v", testResponse, resp)
		}

		// Check logs
		if len(logs) != 2 {
			t.Errorf("Expected 2 log entries, got %d", len(logs))
		}
		if !strings.Contains(logs[0], "Starting request: TestMethod") {
			t.Errorf("Expected start log, got %s", logs[0])
		}
		if !strings.Contains(logs[1], "Request completed: TestMethod") {
			t.Errorf("Expected completion log, got %s", logs[1])
		}

		// Test failed request
		logs = nil
		_, err = interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			return nil, errors.New("test error")
		})

		if err == nil {
			t.Error("Expected error")
		}
		if !strings.Contains(logs[1], "Request failed") {
			t.Errorf("Expected failure log, got %s", logs[1])
		}
	})

	t.Run("TimeoutInterceptor", func(t *testing.T) {
		interceptor := &TimeoutInterceptor{Timeout: 50 * time.Millisecond}

		// Test normal request
		resp, err := interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			return testResponse, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp != testResponse {
			t.Errorf("Expected '%s', got %v", testResponse, resp)
		}

		// Test timeout
		_, err = interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			time.Sleep(100 * time.Millisecond)
			return testResponse, nil
		})

		if err == nil {
			t.Error("Expected timeout error")
		}
		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("Expected timeout error, got %v", err)
		}
	})

	t.Run("RecoveryInterceptor", func(t *testing.T) {
		interceptor := &RecoveryInterceptor{}

		// Test panic recovery
		_, err := interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			panic("test panic")
		})

		if err == nil {
			t.Error("Expected error from panic")
		}
		if !strings.Contains(err.Error(), "panic recovered") {
			t.Errorf("Expected panic recovery error, got %v", err)
		}
	})

	t.Run("MetricsInterceptor", func(t *testing.T) {
		interceptor := &MetricsInterceptor{}

		// Successful request
		_, err := interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			return testResponse, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if interceptor.RequestCount != 1 {
			t.Errorf("Expected RequestCount=1, got %d", interceptor.RequestCount)
		}
		if interceptor.SuccessCount != 1 {
			t.Errorf("Expected SuccessCount=1, got %d", interceptor.SuccessCount)
		}

		// Failed request
		_, err = interceptor.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			return nil, errors.New("test error")
		})

		if err == nil {
			t.Error("Expected error")
		}
		if interceptor.RequestCount != 2 {
			t.Errorf("Expected RequestCount=2, got %d", interceptor.RequestCount)
		}
		if interceptor.FailureCount != 1 {
			t.Errorf("Expected FailureCount=1, got %d", interceptor.FailureCount)
		}
	})

	t.Run("ChainedInterceptors", func(t *testing.T) {
		var order []string

		interceptor1 := &testInterceptor{name: "first", order: &order}
		interceptor2 := &testInterceptor{name: "second", order: &order}

		chained := ChainInterceptors(interceptor1, interceptor2)

		_, err := chained.Intercept(context.Background(), "TestMethod", "request", func(ctx context.Context, req any) (any, error) {
			order = append(order, "handler")
			return testResponse, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Check order: interceptors should wrap in the order they're added
		expected := []string{"first-before", "second-before", "handler", "second-after", "first-after"}
		if len(order) != len(expected) {
			t.Errorf("Expected %d entries, got %d", len(expected), len(order))
		}
		for i, v := range expected {
			if i < len(order) && order[i] != v {
				t.Errorf("Expected order[%d]=%s, got %s", i, v, order[i])
			}
		}
	})
}

// Test helpers
type testWriter struct {
	logs *[]string
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	*w.logs = append(*w.logs, string(p))
	return len(p), nil
}

type testInterceptor struct {
	name  string
	order *[]string
}

func (t *testInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error) {
	*t.order = append(*t.order, t.name+"-before")
	resp, err := handler(ctx, req)
	*t.order = append(*t.order, t.name+"-after")
	return resp, err
}
