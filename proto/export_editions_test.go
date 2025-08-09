package proto

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEditionsSyntaxFix(t *testing.T) {
	tests := []struct {
		name     string
		edition  *descriptorpb.Edition
		expected string
	}{
		{
			name:     "Edition 2023",
			edition:  ptr(descriptorpb.Edition_EDITION_2023),
			expected: `edition = "2023";`,
		},
		{
			name:     "Edition 2024",
			edition:  ptr(descriptorpb.Edition_EDITION_2024),
			expected: `edition = "2024";`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a file descriptor with editions syntax
			fdp := &descriptorpb.FileDescriptorProto{
				Name:    ptr("test.proto"),
				Package: ptr("test.v1"),
				Syntax:  ptr("editions"),
				Edition: tt.edition,
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: ptr("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:     ptr("id"),
								Number:   ptr(int32(1)),
								Type:     ptr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
								Label:    ptr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
								JsonName: ptr("id"),
							},
						},
					},
				},
			}

			// Export the proto
			opts := DefaultExportOptions()
			exporter := NewExporter(&opts)
			content, err := exporter.ExportFileDescriptorProto(fdp)
			if err != nil {
				// Skip if edition is not supported yet
				if strings.Contains(err.Error(), "not yet supported") {
					t.Skipf("Edition not yet supported: %v", err)
				}
				t.Fatalf("Failed to export proto: %v", err)
			}

			// Check that the correct edition syntax is present
			if !strings.Contains(content, tt.expected) {
				t.Errorf("Expected proto to contain %q, but got:\n%s", tt.expected, content)
			}

			// Check that the incorrect syntax is not present
			if strings.Contains(content, `syntax = "editions"`) {
				t.Errorf("Proto should not contain 'syntax = \"editions\"', but got:\n%s", content)
			}
		})
	}
}

func TestEditionsSyntaxFixInFileDescriptorSet(t *testing.T) {
	// Create a FileDescriptorSet with multiple files
	fdset := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    ptr("proto3.proto"),
				Package: ptr("test.v1"),
				Syntax:  ptr("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: ptr("Proto3Message")},
				},
			},
			{
				Name:    ptr("editions.proto"),
				Package: ptr("test.v1"),
				Syntax:  ptr("editions"),
				Edition: ptr(descriptorpb.Edition_EDITION_2023),
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: ptr("EditionsMessage")},
				},
			},
		},
	}

	opts := DefaultExportOptions()
	exporter := NewExporter(&opts)
	files, err := exporter.ExportFileDescriptorSet(fdset)
	if err != nil {
		t.Fatalf("Failed to export FileDescriptorSet: %v", err)
	}

	// Check proto3 file
	if proto3Content, ok := files["proto3.proto"]; ok {
		if !strings.Contains(proto3Content, `syntax = "proto3"`) {
			t.Errorf("Proto3 file should contain 'syntax = \"proto3\"'")
		}
	} else {
		t.Error("proto3.proto not found in exported files")
	}

	// Check editions file
	if editionsContent, ok := files["editions.proto"]; ok {
		if !strings.Contains(editionsContent, `edition = "2023"`) {
			t.Errorf("Editions file should contain 'edition = \"2023\"', got:\n%s", editionsContent)
		}
		if strings.Contains(editionsContent, `syntax = "editions"`) {
			t.Errorf("Editions file should not contain 'syntax = \"editions\"'")
		}
	} else {
		t.Error("editions.proto not found in exported files")
	}
}

// Helper function for creating pointers
func ptr[T any](v T) *T {
	return &v
}
