package compatibility_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/i2y/hyperway/compatibility-tests/testpb"
	"github.com/i2y/hyperway/compatibility-tests/testpb/testpbconnect"
	"github.com/i2y/hyperway/rpc"
)

// TestGRPCWebWithConnectGo tests gRPC-Web protocols using Connect-go client
func TestGRPCWebWithConnectGo(t *testing.T) {
	// Create Hyperway service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	// Register handler - using the protobuf types
	err := rpc.RegisterAs(svc, "Echo", func(ctx context.Context, req *testpb.EchoRequest) (*testpb.EchoResponse, error) {
		return &testpb.EchoResponse{
			Message: "Echo: " + req.Message,
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register method: %v", err)
	}

	// Create handler
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Start test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test cases with Connect-go client
	tests := []struct {
		name    string
		options []connect.ClientOption
	}{
		{
			name: "gRPC-Web+JSON",
			options: []connect.ClientOption{
				connect.WithGRPCWeb(),
				connect.WithProtoJSON(),
			},
		},
		{
			name: "gRPC-Web+Protobuf",
			options: []connect.ClientOption{
				connect.WithGRPCWeb(),
			},
		},
		{
			name: "Connect+JSON",
			options: []connect.ClientOption{
				connect.WithProtoJSON(),
			},
		},
		{
			name:    "Connect+Protobuf",
			options: []connect.ClientOption{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Connect-go client with specific protocol options
			client := testpbconnect.NewTestServiceClient(
				http.DefaultClient,
				server.URL,
				tt.options...,
			)

			// Create request using the generated protobuf types
			req := connect.NewRequest(&testpb.EchoRequest{
				Message: "Hello from " + tt.name,
			})

			// Set timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Call Echo method
			resp, err := client.Echo(ctx, req)
			if err != nil {
				// Log detailed error for debugging
				t.Fatalf("%s failed: %v", tt.name, err)
			}

			// Verify response
			expected := "Echo: Hello from " + tt.name
			if resp.Msg.Message != expected {
				t.Errorf("%s: expected %q, got %q", tt.name, expected, resp.Msg.Message)
			}

			t.Logf("%s: SUCCESS - received %q", tt.name, resp.Msg.Message)
		})
	}
}
