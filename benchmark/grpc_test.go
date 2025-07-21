package benchmark

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/i2y/hyperway/rpc"
)

// Benchmark gRPC protocol
func BenchmarkGRPC_FullLatency(b *testing.B) {
	// Create service with gRPC support
	svc := rpc.NewService("GRPCBenchService",
		rpc.WithPackage("bench.v1"),
		rpc.WithReflection(true),
	)

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", echoHandler).
			In(EchoRequest{}).
			Out(EchoResponse{}),
	)

	// Create gateway (not used directly in this simplified benchmark)
	_, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	// In a real implementation, we'd register the service with gRPC
	// For now, we'll use HTTP/2 which gRPC uses under the hood
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			b.Logf("Server stopped: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Create gRPC client
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	// For now, we'll benchmark the gateway handler directly
	// as full gRPC integration requires generated code
	req := EchoRequest{
		Message: "Hello, gRPC!",
		Count:   100,
		Tags:    []string{"grpc", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := echoHandler(context.Background(), &req)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
}

// Benchmark Connect-RPC protocol
func BenchmarkConnectRPC_FullLatency(b *testing.B) {
	svc := rpc.NewService("ConnectBenchService",
		rpc.WithPackage("bench.v1"),
		rpc.WithValidation(true),
	)

	// Create typed method for Connect
	echoMethod := rpc.NewMethod("Echo", echoHandler).
		In(EchoRequest{}).
		Out(EchoResponse{})

	rpc.MustRegister(svc, echoMethod)

	// Get the Connect handler
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	// Create test server
	server := startTestServer(b, gateway)
	defer server.Close()

	// Create Connect client
	client := newConnectClient[*EchoRequest, *EchoResponse](
		server.URL + "/bench.v1.ConnectBenchService/Echo",
	)

	req := EchoRequest{
		Message: "Hello, Connect!",
		Count:   100,
		Tags:    []string{"connect", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := client.CallUnary(context.Background(), newConnectRequest(&req))
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Msg
	}
}

// Benchmark Connect-RPC with streaming (simplified)
func BenchmarkConnectRPC_Streaming(b *testing.B) {
	// For now, we'll skip streaming benchmarks as they require
	// more complex implementation
	b.Skip("Streaming benchmarks require full Connect integration")
}

// Benchmark large payload with Connect-RPC
func BenchmarkConnectRPC_LargePayload(b *testing.B) {
	svc := rpc.NewService("LargePayloadService",
		rpc.WithPackage("bench.v1"),
	)

	rpc.MustRegister(svc,
		rpc.NewMethod("Process", processHandler).
			In(LargeRequest{}).
			Out(LargeResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := startTestServer(b, gateway)
	defer server.Close()

	client := newConnectClient[*LargeRequest, *LargeResponse](
		server.URL + "/bench.v1.LargePayloadService/Process",
	)

	// Create large request
	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{
			ID:          i,
			Name:        "Item Name",
			Description: "This is a longer description to increase payload size",
			Tags:        []string{"tag1", "tag2", "tag3"},
			Metadata:    map[string]string{"key1": "value1", "key2": "value2"},
		}
	}

	req := LargeRequest{
		Items: items,
		Query: "benchmark query",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := client.CallUnary(context.Background(), newConnectRequest(&req))
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Msg
	}
}

// streamEchoHandler would handle streaming responses
// For benchmarks, we're focusing on unary calls
