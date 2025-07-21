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

// Benchmark raw Connect-style handler (what you'd write without any framework)
func BenchmarkRawConnect_Handler(b *testing.B) {
	// This simulates a hand-written Connect handler without any framework
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Connect protocol headers
		if r.Header.Get("Connect-Protocol-Version") != "1" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Read and unmarshal request
		var req EchoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Process request (business logic)
		resp := EchoResponse{
			Echo:  req.Message,
			Count: req.Count,
			Tags:  req.Tags,
		}

		// Marshal and send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	reqData := EchoRequest{
		Message: "Hello, Raw Connect!",
		Count:   100,
		Tags:    []string{"raw", "connect", "bench"},
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

// Benchmark Hyperway vs raw handler processing time only (no HTTP)
func BenchmarkProcessingOnly_RawHandler(b *testing.B) {
	// Direct function call - baseline
	req := &EchoRequest{
		Message: "Hello, Processing!",
		Count:   100,
		Tags:    []string{"processing", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp := &EchoResponse{
			Echo:  req.Message,
			Count: req.Count,
			Tags:  req.Tags,
		}
		_ = resp
	}
}

func BenchmarkProcessingOnly_Hyperway(b *testing.B) {
	// Hyperway handler call
	req := &EchoRequest{
		Message: "Hello, Processing!",
		Count:   100,
		Tags:    []string{"processing", "bench", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := echoHandler(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
}

// Benchmark with validation
func BenchmarkRawConnect_WithValidation(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EchoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Manual validation (what you'd write without a framework)
		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}
		if req.Count < 0 || req.Count > 1000 {
			http.Error(w, "count must be between 0 and 1000", http.StatusBadRequest)
			return
		}
		if len(req.Tags) > 10 {
			http.Error(w, "too many tags", http.StatusBadRequest)
			return
		}

		resp := EchoResponse{
			Echo:  req.Message,
			Count: req.Count,
			Tags:  req.Tags,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	reqData := EchoRequest{
		Message: "Hello, Validated!",
		Count:   100,
		Tags:    []string{"validated", "bench"},
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

// Benchmark Hyperway with validation enabled
func BenchmarkHyperway_WithValidation(b *testing.B) {
	svc := rpc.NewService("BenchService",
		rpc.WithPackage("bench.v1"),
		rpc.WithValidation(true),
	)

	// Handler with validation tags
	type ValidatedRequest struct {
		Message string   `json:"message" validate:"required"`
		Count   int32    `json:"count" validate:"min=0,max=1000"`
		Tags    []string `json:"tags" validate:"max=10"`
	}

	validatedHandler := func(ctx context.Context, req *ValidatedRequest) (*EchoResponse, error) {
		return &EchoResponse{
			Echo:  req.Message,
			Count: req.Count,
			Tags:  req.Tags,
		}, nil
	}

	rpc.MustRegister(svc,
		rpc.NewMethod("Echo", validatedHandler).
			In(ValidatedRequest{}).
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

	reqData := ValidatedRequest{
		Message: "Hello, Validated!",
		Count:   100,
		Tags:    []string{"validated", "bench"},
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
