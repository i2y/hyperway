// Package rpc provides interceptor implementations.
package rpc

import (
	"context"
	"fmt"
	"log"
	"time"
)

// LoggingInterceptor logs requests and responses.
type LoggingInterceptor struct {
	Logger *log.Logger
}

func (l *LoggingInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error) {
	start := time.Now()
	if l.Logger != nil {
		l.Logger.Printf("Starting request: %s", method)
	}

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	if l.Logger != nil {
		if err != nil {
			l.Logger.Printf("Request failed: %s (duration: %v, error: %v)", method, duration, err)
		} else {
			l.Logger.Printf("Request completed: %s (duration: %v)", method, duration)
		}
	}

	return resp, err
}

// TimeoutInterceptor adds timeout to requests.
type TimeoutInterceptor struct {
	Timeout time.Duration
}

func (t *TimeoutInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error) {
	if t.Timeout <= 0 {
		return handler(ctx, req)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	type result struct {
		resp any
		err  error
	}

	done := make(chan result, 1)
	go func() {
		resp, err := handler(timeoutCtx, req)
		done <- result{resp, err}
	}()

	select {
	case res := <-done:
		return res.resp, res.err
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("request timeout after %v", t.Timeout)
	}
}

// RecoveryInterceptor recovers from panics.
type RecoveryInterceptor struct{}

func (r *RecoveryInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return handler(ctx, req)
}

// MetricsInterceptor collects metrics.
type MetricsInterceptor struct {
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	TotalDuration time.Duration
}

func (m *MetricsInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error) {
	start := time.Now()
	m.RequestCount++

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	m.TotalDuration += duration

	if err != nil {
		m.FailureCount++
	} else {
		m.SuccessCount++
	}

	return resp, err
}

// ChainInterceptors chains multiple interceptors into a single interceptor.
func ChainInterceptors(interceptors ...Interceptor) Interceptor {
	return &chainedInterceptor{interceptors: interceptors}
}

type chainedInterceptor struct {
	interceptors []Interceptor
}

func (c *chainedInterceptor) Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error) {
	// Build the handler chain
	finalHandler := handler

	// Apply interceptors in reverse order
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		interceptor := c.interceptors[i]
		next := finalHandler
		finalHandler = func(ctx context.Context, req any) (any, error) {
			return interceptor.Intercept(ctx, method, req, next)
		}
	}

	return finalHandler(ctx, req)
}
