package schema_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/schema"
)

// Test structs with hyperway:"oneof" tags

type TaggedUser struct {
	UserID string

	// Explicitly tagged oneof
	Identifier struct {
		Email       *string
		PhoneNumber *string
		Username    *string
	} `hyperway:"oneof"`
}

type TaggedMultiOneof struct {
	RequestID string

	// First oneof
	Target struct {
		User         *string
		Group        *string
		Organization *string
	} `hyperway:"oneof"`

	// Second oneof
	Action struct {
		Create *bool
		Update *bool
		Delete *bool
	} `hyperway:"oneof"`
}

// Test proto generation for tagged oneof fields
func TestTaggedOneofProtoGeneration(t *testing.T) {
	tests := []struct {
		name          string
		structType    any
		expectedProto []string
	}{
		{
			name:       "single tagged oneof",
			structType: TaggedUser{},
			expectedProto: []string{
				"message TaggedUser {",
				"string user_i_d = 1;",
				"oneof identifier {",
				"string email = 2;",
				"string phone_number = 3;",
				"string username = 4;",
				"}",
			},
		},
		{
			name:       "multiple tagged oneofs",
			structType: TaggedMultiOneof{},
			expectedProto: []string{
				"message TaggedMultiOneof {",
				"string request_i_d = 1;",
				"oneof target {",
				"string user = 2;",
				"string group = 3;",
				"string organization = 4;",
				"}",
				"oneof action {",
				"bool create = 5;",
				"bool update = 6;",
				"bool delete = 7;",
				"}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build schema
			builder := schema.NewBuilder(schema.BuilderOptions{
				PackageName: "test.v1",
			})

			_, err := builder.BuildMessage(reflect.TypeOf(tt.structType))
			if err != nil {
				t.Fatalf("Failed to build schema: %v", err)
			}

			// Get FileDescriptorSet
			fdset := builder.GetFileDescriptorSet()
			if fdset == nil || len(fdset.File) == 0 {
				t.Fatal("No file descriptor set generated")
			}

			// Export to proto
			exporter := proto.NewExporter(proto.DefaultExportOptions())
			files, err := exporter.ExportFileDescriptorSet(fdset)
			if err != nil {
				t.Fatalf("Failed to export proto: %v", err)
			}

			// Find the proto file
			var protoContent string
			for _, content := range files {
				if strings.Contains(content, "message") {
					protoContent = content
					break
				}
			}

			if protoContent == "" {
				t.Fatal("No proto content generated")
			}

			// Check expected strings
			for _, expected := range tt.expectedProto {
				if !strings.Contains(protoContent, expected) {
					t.Errorf("Expected proto to contain %q, but it didn't.\nProto:\n%s", expected, protoContent)
				}
			}

			// Log the proto for debugging
			t.Logf("Generated proto:\n%s", protoContent)
		})
	}
}

// Test that non-tagged structs don't generate oneofs
func TestNonTaggedNoOneof(t *testing.T) {
	type NonTaggedStruct struct {
		UserID string

		// No tag - should be treated as regular field
		Identifier struct {
			Email       *string
			PhoneNumber *string
		}

		// Even with all pointer fields - no tag means no oneof
		Options struct {
			OptionA *string
			OptionB *string
			OptionC *string
		}
	}

	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	_, err := builder.BuildMessage(reflect.TypeOf(NonTaggedStruct{}))
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	fdset := builder.GetFileDescriptorSet()
	if fdset == nil || len(fdset.File) == 0 {
		t.Fatal("No file descriptor set generated")
	}

	// Check that no oneofs were generated
	for _, file := range fdset.File {
		for _, msg := range file.MessageType {
			if len(msg.OneofDecl) > 0 {
				t.Errorf("Expected no oneofs, but found %d", len(msg.OneofDecl))
			}
		}
	}
}
