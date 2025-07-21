package schema_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_OptionalFields(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type OptionalFieldsMessage struct {
		// Non-pointer fields - implicit optional (no proto3_optional)
		RequiredString string  `json:"required_string"`
		RequiredInt    int32   `json:"required_int"`
		RequiredBool   bool    `json:"required_bool"`
		RequiredFloat  float64 `json:"required_float"`

		// Pointer fields - explicit optional (proto3_optional = true)
		OptionalString *string  `json:"optional_string"`
		OptionalInt    *int32   `json:"optional_int"`
		OptionalBool   *bool    `json:"optional_bool"`
		OptionalFloat  *float64 `json:"optional_float"`

		// Explicit proto tag
		ExplicitOptional string `json:"explicit_optional" proto:"optional"`

		// Repeated fields should not be optional
		RepeatedStrings []string `json:"repeated_strings"`

		// Map fields should not be optional
		MapField map[string]string `json:"map_field"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(OptionalFieldsMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Get the FileDescriptorSet to check proto3_optional
	fdset := builder.GetFileDescriptorSet()
	if len(fdset.File) == 0 {
		t.Fatal("No files in FileDescriptorSet")
	}

	// Find the message descriptor
	var msgDesc *descriptorpb.DescriptorProto
	for _, file := range fdset.File {
		for _, msg := range file.MessageType {
			if msg.GetName() == "OptionalFieldsMessage" {
				msgDesc = msg
				break
			}
		}
	}

	if msgDesc == nil {
		t.Fatal("Could not find OptionalFieldsMessage descriptor")
	}

	// Test cases for each field
	tests := []struct {
		fieldName       string
		expectProto3Opt bool
		expectRepeated  bool
	}{
		// Non-pointer fields should NOT have proto3_optional
		{"required_string", false, false},
		{"required_int", false, false},
		{"required_bool", false, false},
		{"required_float", false, false},

		// Pointer fields SHOULD have proto3_optional
		{"optional_string", true, false},
		{"optional_int", true, false},
		{"optional_bool", true, false},
		{"optional_float", true, false},

		// Explicit proto tag
		{"explicit_optional", true, false},

		// Repeated fields
		{"repeated_strings", false, true},

		// Map fields (represented as repeated map entries)
		{"map_field", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			var fieldDesc *descriptorpb.FieldDescriptorProto
			for _, field := range msgDesc.Field {
				if field.GetName() == tt.fieldName {
					fieldDesc = field
					break
				}
			}

			if fieldDesc == nil {
				t.Fatalf("Field %s not found", tt.fieldName)
			}

			// Check label
			if tt.expectRepeated {
				if fieldDesc.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
					t.Errorf("Field %s: expected LABEL_REPEATED, got %v", tt.fieldName, fieldDesc.GetLabel())
				}
			} else {
				if fieldDesc.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
					t.Errorf("Field %s: expected LABEL_OPTIONAL, got %v", tt.fieldName, fieldDesc.GetLabel())
				}
			}

			// Check proto3_optional
			hasProto3Opt := fieldDesc.GetProto3Optional()
			if hasProto3Opt != tt.expectProto3Opt {
				t.Errorf("Field %s: expected proto3_optional=%v, got %v",
					tt.fieldName, tt.expectProto3Opt, hasProto3Opt)
			}
		})
	}

	// Verify through protoreflect API
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		t.Logf("Field %s: HasOptionalKeyword=%v, HasPresence=%v, Cardinality=%v",
			field.Name(), field.HasOptionalKeyword(), field.HasPresence(), field.Cardinality())
	}
}

func TestBuilder_NestedOptionalFields(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type NestedMessage struct {
		Value string `json:"value"`
	}

	type ParentMessage struct {
		// Non-optional nested message
		RequiredNested NestedMessage `json:"required_nested"`

		// Optional nested message (pointer)
		OptionalNested *NestedMessage `json:"optional_nested"`

		// Repeated nested messages
		RepeatedNested []NestedMessage `json:"repeated_nested"`
	}

	_, err := builder.BuildMessage(reflect.TypeOf(ParentMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Get the FileDescriptorSet
	fdset := builder.GetFileDescriptorSet()
	if len(fdset.File) == 0 {
		t.Fatal("No files in FileDescriptorSet")
	}

	// Find the parent message descriptor
	var msgDesc *descriptorpb.DescriptorProto
	for _, file := range fdset.File {
		for _, msg := range file.MessageType {
			if msg.GetName() == "ParentMessage" {
				msgDesc = msg
				break
			}
		}
	}

	if msgDesc == nil {
		t.Fatal("Could not find ParentMessage descriptor")
	}

	// Check fields
	tests := []struct {
		fieldName       string
		expectProto3Opt bool
		expectRepeated  bool
	}{
		{"required_nested", false, false},
		{"optional_nested", true, false},
		{"repeated_nested", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			var fieldDesc *descriptorpb.FieldDescriptorProto
			for _, field := range msgDesc.Field {
				if field.GetName() == tt.fieldName {
					fieldDesc = field
					break
				}
			}

			if fieldDesc == nil {
				t.Fatalf("Field %s not found", tt.fieldName)
			}

			hasProto3Opt := fieldDesc.GetProto3Optional()
			if hasProto3Opt != tt.expectProto3Opt {
				t.Errorf("Field %s: expected proto3_optional=%v, got %v",
					tt.fieldName, tt.expectProto3Opt, hasProto3Opt)
			}
		})
	}
}
