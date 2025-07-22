package test

import (
	"strings"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

func TestEditionsProtoExport(t *testing.T) {
	// Test struct for editions export
	type EditionsExample struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Count   int32  `json:"count"`
		Enabled bool   `json:"enabled"`
	}

	// Create service with editions syntax
	svc := rpc.NewService("EditionsService",
		rpc.WithPackage("editions.test"),
		rpc.WithEdition("2023"),
	)

	// Register a method
	err := svc.Register(&rpc.Method{
		Name: "TestMethod",
		Handler: func(ctx any, req *EditionsExample) (*EditionsExample, error) {
			return req, nil
		},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Export proto files
	allProtos, err := svc.ExportAllProtos()
	if err != nil {
		t.Fatalf("ExportAllProtos failed: %v", err)
	}

	// Find the proto file
	var protoContent string
	for filename, content := range allProtos {
		t.Logf("File %s:\n%s", filename, content)
		if strings.Contains(content, "message EditionsExample") {
			protoContent = content
			break
		}
	}

	if protoContent == "" {
		t.Fatal("Could not find EditionsExample message in exported protos")
	}

	// Check for editions syntax
	if !strings.Contains(protoContent, `edition = "2023";`) {
		t.Errorf("Expected proto to contain 'edition = \"2023\";'")
	}

	// Ensure it doesn't have the old syntax
	if strings.Contains(protoContent, `syntax = "proto3"`) {
		t.Errorf("Proto should not contain 'syntax = \"proto3\"' when using editions")
	}
	if strings.Contains(protoContent, `syntax = "editions"`) {
		t.Errorf("Proto should not contain 'syntax = \"editions\"' (should be 'edition = \"2023\";')")
	}
}

func TestEditions2024ProtoExport(t *testing.T) {
	// Skip test as Edition 2024 is not yet supported by the Go Protobuf runtime
	t.Skip("Edition 2024 is not yet supported by the Go Protobuf runtime")

	// Once supported, this test would verify Edition 2024 export functionality
	// The implementation is ready in Hyperway, waiting for Go Protobuf runtime support
}
