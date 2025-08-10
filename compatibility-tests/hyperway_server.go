// Package compatibility provides Hyperway server implementation for compatibility testing
package compatibility

import (
	"context"
	"fmt"
	"net/http"

	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/rpc"
)

// CreateHyperwayServer creates a Hyperway server with all test methods registered
func CreateHyperwayServer() (http.Handler, error) {
	// Create service with validation
	svc := rpc.NewService("CompatibilityService",
		rpc.WithPackage("compatibility.v1"),
		rpc.WithValidation(true),
	)

	// Register unary methods
	if err := rpc.Register(svc, "SimpleEcho", SimpleEcho); err != nil {
		return nil, fmt.Errorf("failed to register SimpleEcho: %w", err)
	}
	if err := rpc.Register(svc, "ComplexEcho", ComplexEcho); err != nil {
		return nil, fmt.Errorf("failed to register ComplexEcho: %w", err)
	}
	if err := rpc.Register(svc, "WellKnownEcho", WellKnownEcho); err != nil {
		return nil, fmt.Errorf("failed to register WellKnownEcho: %w", err)
	}
	if err := rpc.Register(svc, "TestError", TestError); err != nil {
		return nil, fmt.Errorf("failed to register TestError: %w", err)
	}

	// Register streaming method with proper types
	if err := rpc.RegisterServerStream[StreamRequest, StreamResponse](svc, "ServerStream",
		func(ctx context.Context, req *StreamRequest, stream rpc.ServerStream[StreamResponse]) error {
			// Call the actual handler
			return ServerStream(ctx, req, stream)
		}); err != nil {
		return nil, fmt.Errorf("failed to register ServerStream: %w", err)
	}

	// Register large streaming method for compression testing
	if err := rpc.RegisterServerStream[StreamRequest, StreamResponse](svc, "ServerStreamLarge",
		func(ctx context.Context, req *StreamRequest, stream rpc.ServerStream[StreamResponse]) error {
			// Call the handler for large messages
			return ServerStreamLarge(ctx, req, stream)
		}); err != nil {
		return nil, fmt.Errorf("failed to register ServerStreamLarge: %w", err)
	}

	// Create handler that supports all protocols
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	return handler, nil
}

// ExportProtoFiles exports the proto files for the compatibility service
func ExportProtoFiles(outputDir string) error {
	// Create service
	svc := rpc.NewService("CompatibilityService",
		rpc.WithPackage("compatibility.v1"),
	)

	// Register all methods
	_ = rpc.Register(svc, "SimpleEcho", SimpleEcho)
	_ = rpc.Register(svc, "ComplexEcho", ComplexEcho)
	_ = rpc.Register(svc, "WellKnownEcho", WellKnownEcho)
	_ = rpc.Register(svc, "TestError", TestError)
	_ = rpc.RegisterServerStream[StreamRequest, StreamResponse](svc, "ServerStream",
		func(ctx context.Context, req *StreamRequest, stream rpc.ServerStream[StreamResponse]) error {
			return ServerStream(ctx, req, stream)
		})

	// Export with Go package option for Connect-go
	options := []proto.ExportOption{
		proto.WithGoPackage("github.com/i2y/hyperway/compatibility-tests/gen;compatibility"),
	}

	files, err := svc.ExportAllProtosWithOptions(options...)
	if err != nil {
		return fmt.Errorf("failed to export protos: %w", err)
	}

	// Write files
	for filename, content := range files {
		fmt.Printf("Generated: %s (%d bytes)\n", filename, len(content))
		// In a real implementation, write to outputDir
	}

	return nil
}
