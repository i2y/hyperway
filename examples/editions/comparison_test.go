package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/i2y/hyperway/rpc"
	"github.com/i2y/hyperway/schema"
)

// Message represents a simple message with optional fields.
type Message struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Priority *int32   `json:"priority,omitempty"` // Optional field
	Tags     []string `json:"tags,omitempty"`
}

// Response represents a simple response.
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handler is a simple echo handler.
func handler(ctx context.Context, req *Message) (*Response, error) {
	return &Response{
		Success: true,
		Message: fmt.Sprintf("Received message: %s", req.Content),
	}, nil
}

func TestProtoComparison(t *testing.T) {
	// Create service with proto3 (default)
	proto3Svc := rpc.NewService("MessageService",
		rpc.WithPackage("test.v1"),
	)
	rpc.MustRegister(proto3Svc,
		rpc.NewMethod("Send", handler),
	)

	// Create service with Editions 2023
	editionsSvc := rpc.NewService("MessageService",
		rpc.WithPackage("test.v1"),
		rpc.WithEdition(schema.Edition2023),
	)
	rpc.MustRegister(editionsSvc,
		rpc.NewMethod("Send", handler),
	)

	// Export proto3 version
	proto3Content, err := proto3Svc.ExportProto()
	if err != nil {
		t.Fatalf("Failed to export proto3: %v", err)
	}

	// Export editions version
	editionsContent, err := editionsSvc.ExportProto()
	if err != nil {
		t.Fatalf("Failed to export editions: %v", err)
	}

	// Print comparison
	fmt.Println("=== Proto3 Version ===")
	fmt.Println(proto3Content)
	fmt.Println("\n=== Editions Version ===")
	fmt.Println(editionsContent)

	// Key differences to observe:
	// 1. Syntax line: "proto3" vs "editions" with edition = "2023"
	// 2. Optional fields: proto3 uses "optional" keyword, editions use field presence feature
	// 3. The generated code is functionally equivalent but uses different mechanisms
}

func TestFeatureComparison(t *testing.T) {
	// Test that both proto3 and editions handle the same message correctly
	testCases := []struct {
		name    string
		service *rpc.Service
	}{
		{
			name: "Proto3",
			service: rpc.NewService("TestService",
				rpc.WithPackage("test.v1"),
			),
		},
		{
			name: "Editions 2023",
			service: rpc.NewService("TestService",
				rpc.WithPackage("test.v1"),
				rpc.WithEdition(schema.Edition2023),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the same method on both services
			rpc.MustRegister(tc.service,
				rpc.NewMethod("Process", handler),
			)

			// Get file descriptor set
			fdset := tc.service.GetFileDescriptorSet()
			if fdset == nil || len(fdset.File) == 0 {
				t.Fatal("No files in FileDescriptorSet")
			}

			// Find the message file
			var messageFile any // Using interface to avoid type-specific checks
			for _, file := range fdset.File {
				if len(file.MessageType) > 0 {
					messageFile = file
					break
				}
			}

			if messageFile == nil {
				t.Fatal("Message file not found")
			}

			// Both should successfully create descriptors for the same types
			t.Logf("%s service created successfully with %d files", tc.name, len(fdset.File))
		})
	}
}
