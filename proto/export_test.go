package proto_test

import (
	"context"
	"strings"
	"testing"

	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/rpc"
)

// Test types
type TestRequest struct {
	Name  string `json:"name" validate:"required"`
	Value int32  `json:"value"`
}

type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func testHandler(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	return &TestResponse{Success: true, Message: req.Name}, nil
}

func TestExportProto(t *testing.T) {
	// Create a test service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	rpc.MustRegisterTyped(svc, "TestMethod", testHandler)

	// Export proto
	protoContent, err := svc.ExportProto()
	if err != nil {
		t.Fatalf("Failed to export proto: %v", err)
	}

	// Verify proto content
	if protoContent == "" {
		t.Error("Expected non-empty proto content")
	}

	// Check for expected content in service file
	expectedStrings := []string{
		"syntax = \"proto3\"",
		"package test.v1",
		"service TestService",
		"rpc TestMethod",
		"message TestRequest",
		"message TestResponse",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(protoContent, expected) {
			t.Errorf("Expected proto to contain %q, but it didn't", expected)
		}
	}

	t.Logf("Generated proto:\n%s", protoContent)
}

func TestExportAllProtos(t *testing.T) {
	// Create a test service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	rpc.MustRegisterTyped(svc, "TestMethod", testHandler)

	// Export all protos
	files, err := svc.ExportAllProtos()
	if err != nil {
		t.Fatalf("Failed to export all protos: %v", err)
	}

	// Verify we got at least one file
	if len(files) == 0 {
		t.Error("Expected at least one proto file")
	}

	// In the current implementation, all messages and services are in a single file
	// Check that we have the main service file
	var hasServiceFile bool

	for filename, content := range files {
		t.Logf("File: %s", filename)

		// All files should have proto3 syntax
		if !strings.Contains(content, "syntax = \"proto3\"") {
			t.Errorf("Expected file %s to contain proto3 syntax", filename)
		}

		// Check specific file content
		if strings.HasSuffix(filename, "test.v1.proto") {
			hasServiceFile = true
			// Should contain service definition
			if !strings.Contains(content, "service TestService") {
				t.Errorf("Expected service file to contain 'service TestService'")
			}
			if !strings.Contains(content, "rpc TestMethod") {
				t.Errorf("Expected service file to contain 'rpc TestMethod'")
			}
			// Should contain message definitions
			if !strings.Contains(content, "message TestRequest") {
				t.Errorf("Expected service file to contain 'message TestRequest'")
			}
			if !strings.Contains(content, "message TestResponse") {
				t.Errorf("Expected service file to contain 'message TestResponse'")
			}
		}
	}

	// Verify the service file was exported
	if !hasServiceFile {
		t.Error("Expected service proto file to be exported")
	}
}

func TestExportToZip(t *testing.T) {
	// Create a test service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	rpc.MustRegisterTyped(svc, "TestMethod", testHandler)

	// Get FileDescriptorSet
	fdset := svc.GetFileDescriptorSet()

	// Create exporter
	exporter := proto.NewExporter(proto.DefaultExportOptions())

	// Export to ZIP
	zipData, err := exporter.ExportToZip(fdset)
	if err != nil {
		t.Fatalf("Failed to export to ZIP: %v", err)
	}

	// Verify ZIP data
	if len(zipData) == 0 {
		t.Error("Expected non-empty ZIP data")
	}

	// ZIP files start with "PK"
	if !strings.HasPrefix(string(zipData[:2]), "PK") {
		t.Error("Expected valid ZIP file format")
	}
}

func TestExportOptions(t *testing.T) {
	// Create a test service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	rpc.MustRegisterTyped(svc, "TestMethod", testHandler)

	// Get FileDescriptorSet
	fdset := svc.GetFileDescriptorSet()

	// Test with different options
	tests := []struct {
		name    string
		options proto.ExportOptions
	}{
		{
			name:    "default options",
			options: proto.DefaultExportOptions(),
		},
		{
			name: "sorted elements",
			options: proto.ExportOptions{
				SortElements: true,
				Indent:       "  ",
			},
		},
		{
			name: "custom indent",
			options: proto.ExportOptions{
				Indent: "\t",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := proto.NewExporter(tt.options)
			files, err := exporter.ExportFileDescriptorSet(fdset)
			if err != nil {
				t.Fatalf("Failed to export with options %v: %v", tt.options, err)
			}

			if len(files) == 0 {
				t.Error("Expected at least one proto file")
			}
		})
	}
}
