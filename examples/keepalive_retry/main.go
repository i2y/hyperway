// Package main demonstrates gRPC keepalive and retry mechanisms.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/gateway"
	"github.com/i2y/hyperway/rpc"
)

// EchoRequest represents an echo request.
type EchoRequest struct {
	Message string `json:"message" validate:"required"`
	// Simulate failure for testing retry
	SimulateFailure bool `json:"simulate_failure,omitempty"`
	FailureCount    int  `json:"failure_count,omitempty"`
}

// EchoResponse represents an echo response.
type EchoResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Attempt   int       `json:"attempt"`
}

// Global counter for retry demonstration
var attemptCounter = make(map[string]int)

// EchoHandler handles echo requests with simulated failures.
func EchoHandler(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	// Track attempts for this message
	attemptCounter[req.Message]++
	attempt := attemptCounter[req.Message]

	fmt.Printf("Processing request (attempt %d): %s\n", attempt, req.Message)

	// Simulate failures for retry testing
	if req.SimulateFailure && attempt <= req.FailureCount {
		fmt.Printf("Simulating failure for attempt %d\n", attempt)
		return nil, &rpc.Error{
			Code:    rpc.CodeUnavailable,
			Message: fmt.Sprintf("Service temporarily unavailable (attempt %d)", attempt),
		}
	}

	// Reset counter on success
	delete(attemptCounter, req.Message)

	return &EchoResponse{
		Message:   fmt.Sprintf("Echo: %s", req.Message),
		Timestamp: time.Now(),
		Attempt:   attempt,
	}, nil
}

func main() {
	// Create service configuration with retry policy
	serviceConfig := rpc.ServiceConfig{
		MethodConfig: []rpc.MethodConfig{
			{
				Name: []rpc.MethodName{
					{
						Service: "example.echo.v1.EchoService",
						Method:  "Echo",
					},
				},
				Timeout: "30s",
				RetryPolicy: &rpc.RetryPolicy{
					MaxAttempts:       4,
					InitialBackoff:    "0.1s",
					MaxBackoff:        "10s",
					BackoffMultiplier: 2.0,
					RetryableStatusCodes: []string{
						"UNAVAILABLE",
						"DEADLINE_EXCEEDED",
					},
				},
			},
		},
		RetryThrottling: &rpc.RetryThrottling{
			MaxTokens:  100,
			TokenRatio: 0.1,
		},
	}

	// Convert to JSON
	configJSON, err := json.MarshalIndent(serviceConfig, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal service config: %v", err)
	}

	fmt.Println("Service Configuration:")
	fmt.Println(string(configJSON))
	fmt.Println()

	// Create service with retry configuration
	svc := rpc.NewService("EchoService",
		rpc.WithPackage("example.echo.v1"),
		rpc.WithValidation(true),
		rpc.WithServiceConfig(string(configJSON)),
	)

	// Create retry interceptor
	retryInterceptor := rpc.NewRetryInterceptor(&serviceConfig)

	// Register method with retry interceptor
	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", EchoHandler).
			WithInterceptors(retryInterceptor),
	)

	// Configure keepalive parameters
	keepaliveParams := gateway.AggressiveKeepaliveParams() // For demo purposes
	keepaliveEnforcement := gateway.KeepaliveEnforcementPolicy{
		MinTime:             10 * time.Second, // Allow faster pings for demo
		PermitWithoutStream: true,
		MaxPingStrikes:      5,
	}

	// Create gateway with keepalive configuration
	gatewayOpts := gateway.Options{
		EnableReflection:           true,
		KeepaliveParams:            &keepaliveParams,
		KeepaliveEnforcementPolicy: &keepaliveEnforcement,
		CORSConfig:                 gateway.DefaultCORSConfig(),
	}

	gw, err := gateway.New([]*gateway.Service{
		{
			Name:     svc.Name(),
			Package:  svc.PackageName(),
			Handlers: svc.Handlers(),
		},
	}, gatewayOpts)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Create HTTP/2 server with keepalive
	server := gateway.NewHTTP2Server(":8080", gw, gatewayOpts)

	fmt.Println("Server Configuration:")
	fmt.Printf("- Keepalive Time: %v\n", keepaliveParams.Time)
	fmt.Printf("- Keepalive Timeout: %v\n", keepaliveParams.Timeout)
	fmt.Printf("- Min Ping Interval: %v\n", keepaliveEnforcement.MinTime)
	fmt.Printf("- Max Retry Attempts: %d\n", serviceConfig.MethodConfig[0].RetryPolicy.MaxAttempts)
	fmt.Println()

	fmt.Println("Server running on http://localhost:8080")
	fmt.Println()
	fmt.Println("Test commands:")
	fmt.Println("1. Normal request:")
	fmt.Println(`   curl -X POST http://localhost:8080/example.echo.v1.EchoService/Echo \`)
	fmt.Println(`        -H "Content-Type: application/json" \`)
	fmt.Println(`        -d '{"message":"Hello World"}'`)
	fmt.Println()
	fmt.Println("2. Request with retry (simulates 2 failures):")
	fmt.Println(`   curl -X POST http://localhost:8080/example.echo.v1.EchoService/Echo \`)
	fmt.Println(`        -H "Content-Type: application/json" \`)
	fmt.Println(`        -d '{"message":"Test Retry","simulate_failure":true,"failure_count":2}'`)
	fmt.Println()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
