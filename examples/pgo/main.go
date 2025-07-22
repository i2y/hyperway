// Package main demonstrates Profile-Guided Optimization (PGO) with hyperway.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/codec"
	"github.com/i2y/hyperway/internal/proto"
	"github.com/i2y/hyperway/rpc"
)

// ComplexMessage represents a complex message for PGO demonstration
type ComplexMessage struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data"`
	Items     []Item            `json:"items"`
	Metadata  *Metadata         `json:"metadata,omitempty"`
}

type Item struct {
	Name  string   `json:"name"`
	Value float64  `json:"value"`
	Tags  []string `json:"tags"`
}

type Metadata struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func processMessage(ctx context.Context, req *ComplexMessage) (*Response, error) {
	// Simulate processing
	return &Response{
		Success: true,
		Message: fmt.Sprintf("Processed message %s with %d items", req.ID, len(req.Items)),
	}, nil
}

func main() {
	// Create service with PGO enabled
	svc := rpc.NewService("PGODemo",
		rpc.WithPackage("pgo.v1"),
		rpc.WithValidation(false), // Disable validation for performance testing
	)

	// Register method
	if err := rpc.Register(svc, "ProcessMessage", processMessage); err != nil {
		log.Fatalf("Failed to register ProcessMessage: %v", err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	srv := &http.Server{
		Addr:    ":8090",
		Handler: gateway,
	}

	log.Println("PGO Demo server starting on :8090")
	log.Println("The server will collect profiles for the first 30 seconds,")
	log.Println("then recompile with PGO for optimized performance.")

	// Schedule PGO recompilation after 30 seconds
	go func() {
		time.Sleep(30 * time.Second)
		log.Println("Recompiling message types with collected profiles...")

		if err := proto.GlobalPGOManager.RecompileAll(); err != nil {
			log.Printf("Failed to recompile with PGO: %v", err)
		} else {
			log.Println("Successfully recompiled with PGO! Performance should now be optimized.")
		}
	}()

	// Enable PGO in codec options
	codec.DefaultDecoderOptions.EnablePGO = true

	log.Fatal(srv.ListenAndServe())
}

// To test this example:
// 1. Start the server: go run examples/pgo/main.go
// 2. Send many requests during the first 30 seconds to build a profile:
//    for i in {1..1000}; do
//      curl -X POST http://localhost:8090/pgo.v1.PGODemo/ProcessMessage \
//        -H "Content-Type: application/json" \
//        -d '{
//          "id": "msg-'$i'",
//          "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
//          "data": {"key1": "value1", "key2": "value2"},
//          "items": [
//            {"name": "item1", "value": 123.45, "tags": ["tag1", "tag2"]},
//            {"name": "item2", "value": 678.90, "tags": ["tag3"]}
//          ],
//          "metadata": {"version": 1, "source": "test"}
//        }' &
//    done
// 3. After 30 seconds, the server will recompile with PGO
// 4. Continue sending requests to see optimized performance
