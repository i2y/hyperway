package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Benchmark standard HTTP handler for comparison
func BenchmarkStandardHTTP_FullLatency(b *testing.B) {
	// Standard HTTP handler without Hyperway
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EchoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
		Message: "Hello, Standard HTTP!",
		Count:   100,
		Tags:    []string{"bench", "test", "latency"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/echo",
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

// Benchmark standard HTTP with validation
func BenchmarkStandardHTTP_WithValidation(b *testing.B) {
	// Standard HTTP handler with manual validation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EchoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Manual validation
		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}
		if req.Count < 0 {
			http.Error(w, "count must be positive", http.StatusBadRequest)
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
		Message: "Hello with validation!",
		Count:   100,
		Tags:    []string{"bench", "test", "validation"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/echo",
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
