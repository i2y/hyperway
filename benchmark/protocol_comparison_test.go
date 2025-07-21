package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

// Benchmark JSON protocol (REST-like)
func BenchmarkProtocol_JSON(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", echoHandler).
			In(EchoRequest{}).
			Out(EchoResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	reqData := EchoRequest{
		Message: "Hello, JSON!",
		Count:   100,
		Tags:    []string{"json", "bench", "test"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.BenchService/Echo",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Got status %d: %s", resp.StatusCode, body)
		}
	}
}

// Benchmark Connect protocol (Protobuf over HTTP)
func BenchmarkProtocol_Connect(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", echoHandler).
			In(EchoRequest{}).
			Out(EchoResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	// Create protobuf request
	reqMsg := &EchoRequest{
		Message: "Hello, Connect!",
		Count:   100,
		Tags:    []string{"connect", "bench", "test"},
	}

	// In Connect protocol, we send protobuf with specific headers
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Marshal to JSON for Connect (Connect supports both JSON and Protobuf)
		reqBody, err := json.Marshal(reqMsg)
		if err != nil {
			b.Fatal(err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.BenchService/Echo",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Got status %d: %s", resp.StatusCode, body)
		}
	}
}

// Benchmark gRPC-Web protocol
func BenchmarkProtocol_GRPCWeb(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", echoHandler).
			In(EchoRequest{}).
			Out(EchoResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	// For gRPC-Web, we need to simulate the protocol
	// This is a simplified version for benchmarking
	reqMsg := &EchoRequest{
		Message: "Hello, gRPC-Web!",
		Count:   100,
		Tags:    []string{"grpc-web", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// gRPC-Web uses base64-encoded protobuf
		reqBody, err := json.Marshal(reqMsg) // Simplified - would use protobuf
		if err != nil {
			b.Fatal(err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.BenchService/Echo",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/grpc-web+json")
		req.Header.Set("X-Grpc-Web", "1")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Got status %d: %s", resp.StatusCode, body)
		}
	}
}

// Benchmark with actual protobuf encoding
func BenchmarkProtocol_ConnectProtobuf(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", echoHandler).
			In(EchoRequest{}).
			Out(EchoResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	// For proper protobuf encoding, we would need the message descriptor
	// but for benchmarking purposes, we'll use JSON

	// For now, we'll use JSON encoding as protobuf requires descriptor
	reqMsg := &EchoRequest{
		Message: "Hello, Protobuf!",
		Count:   100,
		Tags:    []string{"protobuf", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Connect can use either JSON or Protobuf
		// Using JSON for simplicity in benchmarks
		reqBody, err := json.Marshal(reqMsg)
		if err != nil {
			b.Fatal(err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.BenchService/Echo",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/proto")
		req.Header.Set("Connect-Protocol-Version", "1")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Got status %d: %s", resp.StatusCode, body)
		}
	}
}
