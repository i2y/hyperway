package rpc_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

// TestMessageSizeLimits tests message size limit enforcement
func TestMessageSizeLimits(t *testing.T) {
	t.Run("DefaultLimits", func(t *testing.T) {
		// Create service with default limits (4MB)
		svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

		// Register a simple echo handler
		err := rpc.Register(svc, "Echo", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
			return &TestResponse{Message: req.Message}, nil
		})
		if err != nil {
			t.Fatalf("failed to register method: %v", err)
		}

		gateway, err := rpc.NewGateway(svc)
		if err != nil {
			t.Fatalf("failed to create gateway: %v", err)
		}

		server := httptest.NewServer(gateway)
		defer server.Close()

		// Test that a normal message works
		smallMsg := `{"message": "hello"}`
		resp, err := sendRequest(server.URL+"/test.v1.TestService/Echo", smallMsg)
		if err != nil {
			t.Fatalf("failed to send small message: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		// Test that a message exceeding 4MB is rejected
		largeMsg := `{"message": "` + strings.Repeat("x", 4*1024*1024) + `"}`
		resp, err = sendRequest(server.URL+"/test.v1.TestService/Echo", largeMsg)
		if err != nil {
			t.Fatalf("failed to send large message: %v", err)
		}
		defer resp.Body.Close()

		// Should get a 429 Resource Exhausted error
		if resp.StatusCode != http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 429 for oversized message, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("CustomReceiveLimit", func(t *testing.T) {
		// Create service with custom receive limit (100KB like connecpy)
		svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"),
			rpc.WithMaxReceiveMessageSize(100*1024), // 100KB
		)

		err := rpc.Register(svc, "Echo", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
			return &TestResponse{Message: req.Message}, nil
		})
		if err != nil {
			t.Fatalf("failed to register method: %v", err)
		}

		gateway, err := rpc.NewGateway(svc)
		if err != nil {
			t.Fatalf("failed to create gateway: %v", err)
		}

		server := httptest.NewServer(gateway)
		defer server.Close()

		// Test that a message under 100KB works
		smallMsg := `{"message": "` + strings.Repeat("x", 50*1024) + `"}`
		resp, err := sendRequest(server.URL+"/test.v1.TestService/Echo", smallMsg)
		if err != nil {
			t.Fatalf("failed to send 50KB message: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 200 for 50KB message, got %d, body: %s", resp.StatusCode, body)
		}
		resp.Body.Close()

		// Test that a message exceeding 100KB is rejected
		largeMsg := `{"message": "` + strings.Repeat("x", 150*1024) + `"}`
		resp, err = sendRequest(server.URL+"/test.v1.TestService/Echo", largeMsg)
		if err != nil {
			t.Fatalf("failed to send 150KB message: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 429 for 150KB message with 100KB limit, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("CustomSendLimit", func(t *testing.T) {
		// Create service with custom send limit
		svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"),
			rpc.WithMaxSendMessageSize(1024), // 1KB send limit
		)

		// Register handler that returns large response
		err := rpc.Register(svc, "GetLarge", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
			// Try to return a response larger than 1KB
			return &TestResponse{Message: strings.Repeat("x", 2000)}, nil
		})
		if err != nil {
			t.Fatalf("failed to register method: %v", err)
		}

		gateway, err := rpc.NewGateway(svc)
		if err != nil {
			t.Fatalf("failed to create gateway: %v", err)
		}

		server := httptest.NewServer(gateway)
		defer server.Close()

		// Send request that triggers large response
		resp, err := sendRequest(server.URL+"/test.v1.TestService/GetLarge", `{"message": "trigger"}`)
		if err != nil {
			t.Fatalf("failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Should get an error because response exceeds send limit
		if resp.StatusCode == http.StatusOK {
			t.Errorf("expected error for oversized response, got status 200")
		}
	})

	t.Run("StreamingSendLimit", func(t *testing.T) {
		// Create service with custom send limit for streaming
		svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"),
			rpc.WithMaxSendMessageSize(1024), // 1KB send limit
		)

		// Register server-streaming handler
		err := rpc.RegisterServerStream(svc, "StreamLarge",
			func(ctx context.Context, req *TestRequest, stream rpc.ServerStream[TestResponse]) error {
				// Try to send a message larger than 1KB
				largeResp := &TestResponse{Message: strings.Repeat("x", 2000)}
				return stream.Send(largeResp)
			})
		if err != nil {
			t.Fatalf("failed to register streaming method: %v", err)
		}

		gateway, err := rpc.NewGateway(svc)
		if err != nil {
			t.Fatalf("failed to create gateway: %v", err)
		}

		server := httptest.NewServer(gateway)
		defer server.Close()

		// Send request to streaming endpoint
		resp, err := sendRequest(server.URL+"/test.v1.TestService/StreamLarge", `{"message": "trigger"}`)
		if err != nil {
			t.Fatalf("failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Read response - should contain error about message size
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// For streaming, we might get an empty response if the stream errors immediately
		// Check response status or body for error
		if resp.StatusCode == http.StatusOK && bodyStr == "" {
			// Stream might have terminated without sending the error in the body
			// This is acceptable behavior for a streaming error
			t.Logf("Stream terminated with empty body, likely due to send size limit")
		} else if !strings.Contains(bodyStr, "exceeds maximum send size") &&
			!strings.Contains(bodyStr, "resource_exhausted") &&
			!strings.Contains(bodyStr, "RESOURCE_EXHAUSTED") {
			// If we got a response body, it should contain the error
			if bodyStr != "" {
				t.Errorf("expected resource exhausted error in response, got: %s", bodyStr)
			}
		}
	})

	t.Run("CompressionWithSizeLimit", func(t *testing.T) {
		// Test that compressed messages are checked after decompression
		svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"),
			rpc.WithMaxReceiveMessageSize(100*1024), // 100KB limit
		)

		err := rpc.Register(svc, "Echo", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
			return &TestResponse{Message: req.Message}, nil
		})
		if err != nil {
			t.Fatalf("failed to register method: %v", err)
		}

		gateway, err := rpc.NewGateway(svc)
		if err != nil {
			t.Fatalf("failed to create gateway: %v", err)
		}

		server := httptest.NewServer(gateway)
		defer server.Close()

		// Create a message that compresses well but exceeds limit when decompressed
		// Repeating patterns compress very well
		largeMsg := `{"message": "` + strings.Repeat("abc", 50*1024) + `"}`

		// Compress the message with gzip
		compressor, ok := rpc.GetCompressor(rpc.CompressionGzip)
		if !ok {
			t.Skip("gzip compressor not available")
		}

		compressed, err := compressor.Compress([]byte(largeMsg))
		if err != nil {
			t.Fatalf("failed to compress message: %v", err)
		}

		// Compressed size should be much smaller than original
		if len(compressed) > 10*1024 {
			t.Logf("Warning: compressed size %d is larger than expected", len(compressed))
		}

		// Send compressed request
		req, err := http.NewRequest("POST", server.URL+"/test.v1.TestService/Echo", bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to send compressed request: %v", err)
		}
		defer resp.Body.Close()

		// Should be rejected because decompressed size exceeds limit
		if resp.StatusCode != http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 429 for oversized decompressed message, got %d, body: %s",
				resp.StatusCode, body)
		}
	})
}

// TestRequest is a test request type
type TestRequest struct {
	Message string `json:"message"`
}

// TestResponse is a test response type
type TestResponse struct {
	Message string `json:"message"`
}

// sendRequest sends a POST request with JSON body
func sendRequest(url, body string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

// TestGRPCMessageSizeLimits tests message size limits for gRPC protocol
func TestGRPCMessageSizeLimits(t *testing.T) {
	// Create service with custom limits
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"),
		rpc.WithMaxReceiveMessageSize(1024), // 1KB receive limit
		rpc.WithMaxSendMessageSize(2048),    // 2KB send limit
	)

	err := rpc.Register(svc, "Echo", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return &TestResponse{Message: req.Message}, nil
	})
	if err != nil {
		t.Fatalf("failed to register method: %v", err)
	}

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	t.Run("gRPCReceiveLimit", func(t *testing.T) {
		// Create a gRPC frame with message larger than 1KB
		largeMsg := strings.Repeat("x", 2000)
		msgBytes := []byte(fmt.Sprintf(`{"message":"%s"}`, largeMsg))

		// Build gRPC frame: 1 byte flags + 4 bytes length + message
		frame := make([]byte, 5+len(msgBytes))
		frame[0] = 0 // no compression
		// Set length (big-endian)
		frame[1] = byte(len(msgBytes) >> 24)
		frame[2] = byte(len(msgBytes) >> 16)
		frame[3] = byte(len(msgBytes) >> 8)
		frame[4] = byte(len(msgBytes))
		copy(frame[5:], msgBytes)

		// Send gRPC request
		req, err := http.NewRequest("POST", server.URL+"/test.v1.TestService/Echo", bytes.NewReader(frame))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/grpc+json")
		req.Header.Set("TE", "trailers")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to send gRPC request: %v", err)
		}
		defer resp.Body.Close()

		// Check for resource exhausted error in trailers
		grpcStatus := resp.Header.Get("grpc-status")
		if grpcStatus == "" {
			// Status might be in trailers
			grpcStatus = resp.Trailer.Get("grpc-status")
		}

		// Status 8 is RESOURCE_EXHAUSTED in gRPC
		if grpcStatus != "8" {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected grpc-status 8 (RESOURCE_EXHAUSTED), got %s, body: %s", grpcStatus, body)
		}
	})
}
