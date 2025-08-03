package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/i2y/hyperway/rpc"
)

// Request/Response types
type CalcRequest struct {
	A int `json:"a" validate:"required"`
	B int `json:"b" validate:"required"`
}

type CalcResponse struct {
	Result int `json:"result"`
}

// Handler functions
func add(ctx context.Context, req *CalcRequest) (*CalcResponse, error) {
	return &CalcResponse{
		Result: req.A + req.B,
	}, nil
}

func multiply(ctx context.Context, req *CalcRequest) (*CalcResponse, error) {
	return &CalcResponse{
		Result: req.A * req.B,
	}, nil
}

func subtract(ctx context.Context, req *CalcRequest) (*CalcResponse, error) {
	return &CalcResponse{
		Result: req.A - req.B,
	}, nil
}

func main() {
	// Create a service with JSON-RPC enabled
	svc := rpc.NewService("CalculatorService",
		rpc.WithPackage("calculator.v1"),
		rpc.WithValidation(true),
		rpc.WithJSONRPC("/api/jsonrpc"), // Enable JSON-RPC at /api/jsonrpc
		rpc.WithJSONRPCBatchLimit(50),   // Limit batch requests
		rpc.WithReflection(true),        // Enable gRPC reflection
		rpc.WithDescription("Multi-protocol calculator service (HTTP/2)"),
	)

	// Register methods
	rpc.MustRegister(svc, "add", add)
	rpc.MustRegister(svc, "multiply", multiply)
	rpc.MustRegister(svc, "subtract", subtract)

	// Create gateway - this supports gRPC, Connect, gRPC-Web, and JSON-RPC
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Create HTTP/2 server with h2c (HTTP/2 without TLS)
	h2s := &http2.Server{}

	// Create handler that supports both HTTP/1.1 and HTTP/2
	handler := h2c.NewHandler(gateway, h2s)

	addr := ":8084"
	server := &http.Server{
		Addr:      addr,
		Handler:   handler,
		TLSConfig: &tls.Config{
			// For production, use proper certificates
			// This is just for testing HTTP/2
		},
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting HTTP/2 server on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  - JSON-RPC: http://localhost%s/api/jsonrpc", addr)
	log.Printf("  - gRPC/Connect: http://localhost%s/calculator.v1.CalculatorService/[method]", addr)
	log.Printf("Server supports: HTTP/1.1, HTTP/2 (h2c), gRPC, Connect-RPC, gRPC-Web, JSON-RPC")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
