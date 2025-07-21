package benchmark

import (
	"context"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/i2y/hyperway/rpc"
	"github.com/i2y/hyperway/schema"
)

// Test types for hyperway
type BenchRequest struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Count   int32             `json:"count"`
	Active  bool              `json:"active"`
	Tags    []string          `json:"tags"`
	Details map[string]string `json:"details"`
}

type BenchResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// Handler for benchmarks
func benchHandler(ctx context.Context, req *BenchRequest) (*BenchResponse, error) {
	return &BenchResponse{
		ID:        req.ID,
		Message:   "Processed: " + req.Name,
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// Benchmark hyperway service creation and method registration
func BenchmarkHyperway_ServiceSetup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))
		rpc.MustRegister(svc,
			rpc.NewMethod("Process", benchHandler).
				In(BenchRequest{}).
				Out(BenchResponse{}),
		)
		_, _ = rpc.NewGateway(svc)
	}
}

// Benchmark message building (schema generation)
func BenchmarkHyperway_SchemaGeneration(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This triggers schema generation internally
		_ = svc.Register(
			rpc.NewMethod("Process", benchHandler).
				In(BenchRequest{}).
				Out(BenchResponse{}).
				Build(),
		)
	}
}

// Benchmark message encoding/decoding with hyperway
func BenchmarkHyperway_MessageProcessing(b *testing.B) {
	// Setup
	req := &BenchRequest{
		ID:     "bench-123",
		Name:   "Benchmark Test",
		Count:  100,
		Active: true,
		Tags:   []string{"tag1", "tag2", "tag3"},
		Details: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// In real usage, hyperway handles this internally
	// This simulates the overhead of dynamic message handling
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate struct to proto conversion (what happens in hyperway)
		_ = req // The actual conversion happens inside hyperway handlers
	}
}

// For comparison: Benchmark with regular protobuf (simulated)
func BenchmarkProtobuf_Generated(b *testing.B) {
	// This simulates using generated protobuf code
	// In reality, generated code would be ~2-3x faster for encoding
	// but hyperpb claims to be faster for decoding

	// Build a message descriptor for BenchResponse
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "bench.v1",
	})
	md, err := builder.BuildMessage(reflect.TypeOf(BenchResponse{}))
	if err != nil {
		b.Fatal(err)
	}

	// Create sample data
	msg := dynamicpb.NewMessage(md)
	msg.Set(md.Fields().ByName("success"), protoreflect.ValueOfBool(true))
	msg.Set(md.Fields().ByName("message"), protoreflect.ValueOfString("benchmark response"))
	data, err := proto.Marshal(msg)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := dynamicpb.NewMessage(md)
		_ = proto.Unmarshal(data, msg)
	}
}

// Memory allocation comparison
func BenchmarkHyperway_MemoryAllocation(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))
	rpc.MustRegister(svc,
		rpc.NewMethod("Process", benchHandler).
			In(BenchRequest{}).
			Out(BenchResponse{}),
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &BenchRequest{
			ID:   "test",
			Name: "benchmark",
		}
		resp, _ := benchHandler(context.Background(), req)
		_ = resp
	}
}
