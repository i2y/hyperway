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

// Benchmark full HTTP request/response cycle
func BenchmarkHTTP_FullLatency_JSON(b *testing.B) {
	svc := rpc.NewService("LatencyService", rpc.WithPackage("bench.v1"))

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
		Message: "Hello, Hyperway!",
		Count:   100,
		Tags:    []string{"bench", "test", "latency"},
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.LatencyService/Echo",
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

// Benchmark with larger payload
func BenchmarkHTTP_FullLatency_LargePayload(b *testing.B) {
	svc := rpc.NewService("LatencyService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("Process", processHandler).
			In(LargeRequest{}).
			Out(LargeResponse{}),
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

	// Create a larger payload
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

	reqData := LargeRequest{
		Items: items,
		Query: "benchmark query",
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.LatencyService/Process",
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

// Benchmark parallel requests
func BenchmarkHTTP_ParallelRequests(b *testing.B) {
	svc := rpc.NewService("LatencyService", rpc.WithPackage("bench.v1"))

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
		Message: "Parallel test",
		Count:   10,
	}
	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, err := http.NewRequestWithContext(context.Background(), "POST",
				server.URL+"/bench.v1.LatencyService/Echo",
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
				b.Fatalf("Got status %d", resp.StatusCode)
			}
		}
	})
}

// Test types for latency benchmarks
type EchoRequest struct {
	Message string   `json:"message"`
	Count   int32    `json:"count"`
	Tags    []string `json:"tags,omitempty"`
}

type EchoResponse struct {
	Echo  string   `json:"echo"`
	Count int32    `json:"count"`
	Tags  []string `json:"tags,omitempty"`
}

type Item struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
}

type LargeRequest struct {
	Items []Item `json:"items"`
	Query string `json:"query"`
}

type LargeResponse struct {
	ProcessedCount int32  `json:"processed_count"`
	Status         string `json:"status"`
}

// Handlers
func echoHandler(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{
		Echo:  req.Message,
		Count: req.Count,
		Tags:  req.Tags,
	}, nil
}

func processHandler(ctx context.Context, req *LargeRequest) (*LargeResponse, error) {
	return &LargeResponse{
		ProcessedCount: int32(len(req.Items)),
		Status:         "processed",
	}, nil
}
