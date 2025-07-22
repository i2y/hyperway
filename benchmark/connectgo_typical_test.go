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

// This simulates typical connect-go usage with generated code
// In real usage, you would:
// 1. Write a .proto file
// 2. Generate code with protoc-gen-connect-go
// 3. Implement the generated interface

// Simulated generated types (what protoc-gen-connect-go would generate)
type EchoServiceHandler interface {
	Echo(context.Context, *EchoRequest) (*EchoResponse, error)
}

// Simulated generated server implementation
type echoServiceServer struct {
	// This would implement the generated interface
}

func (s *echoServiceServer) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	// Business logic
	return &EchoResponse{
		Echo:  req.Message,
		Count: req.Count,
		Tags:  req.Tags,
	}, nil
}

// Benchmark typical connect-go with generated code pattern
func BenchmarkConnectGo_Typical(b *testing.B) {
	// This simulates the typical connect-go setup with generated code

	// Create service implementation
	server := &echoServiceServer{}

	// In real connect-go, you'd use NewEchoServiceHandler from generated code
	// This creates all the HTTP handlers for you
	mux := http.NewServeMux()

	// Simulated generated handler (what NewEchoServiceHandler would create)
	mux.HandleFunc("/echo.v1.EchoService/Echo", func(w http.ResponseWriter, r *http.Request) {
		// This is what the generated connect handler does:
		// 1. Check content type and protocol
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" && !isConnectProtocol(r) && !isGRPCWeb(r) {
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		// 2. Decode request based on protocol
		var req EchoRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 3. Call the actual service implementation
		resp, err := server.Echo(r.Context(), &req)
		if err != nil {
			// In real connect-go, this would encode the error properly
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 4. Encode response based on protocol
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	reqData := EchoRequest{
		Message: "Hello, Connect-Go!",
		Count:   100,
		Tags:    []string{"connect-go", "typical", "bench"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			httpServer.URL+"/echo.v1.EchoService/Echo",
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

// Benchmark Hyperway for direct comparison
func BenchmarkHyperway_Equivalent(b *testing.B) {
	// Hyperway setup - no .proto files needed
	svc := rpc.NewService("EchoService", rpc.WithPackage("echo.v1"))

	rpc.MustRegisterMethod(svc,
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
		Message: "Hello, Hyperway!",
		Count:   100,
		Tags:    []string{"hyperway", "dynamic", "bench"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/echo.v1.EchoService/Echo",
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

// Helper functions to check protocols
func isConnectProtocol(r *http.Request) bool {
	return r.Header.Get("Connect-Protocol-Version") == "1"
}

func isGRPCWeb(r *http.Request) bool {
	return r.Header.Get("Content-Type") == "application/grpc-web" ||
		r.Header.Get("Content-Type") == "application/grpc-web+proto" ||
		r.Header.Get("Content-Type") == "application/grpc-web+json"
}

// Benchmark with interceptors/middleware (common in connect-go)
func BenchmarkConnectGo_WithInterceptor(b *testing.B) {
	server := &echoServiceServer{}

	mux := http.NewServeMux()

	// Simulated interceptor pattern (common in connect-go)
	mux.HandleFunc("/echo.v1.EchoService/Echo", func(w http.ResponseWriter, r *http.Request) {
		// Pre-processing (interceptor)
		startTime := timeNow()

		// Decode request
		var req EchoRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		// Call service
		resp, err := server.Echo(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Post-processing (interceptor)
		_ = timeSince(startTime) // logging would happen here

		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	reqData := EchoRequest{
		Message: "Hello, Intercepted!",
		Count:   100,
		Tags:    []string{"interceptor", "bench"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			httpServer.URL+"/echo.v1.EchoService/Echo",
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

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b.Fatal("Got non-200 status")
		}
	}
}

// Dummy time functions to avoid importing time
func timeNow() int64          { return 0 }
func timeSince(_ int64) int64 { return 0 }
