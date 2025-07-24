package schema_test

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_NewWellKnownTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	// Test message with new Well-Known Types
	type NewWellKnownMessage struct {
		// Struct types
		Settings   *structpb.Struct       `json:"settings"`
		Config     *structpb.Value        `json:"config"`
		Values     *structpb.ListValue    `json:"values"`
		UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"`

		// Regular field for comparison
		Name string `json:"name"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(NewWellKnownMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Get the FileDescriptorSet
	fdset := builder.GetFileDescriptorSet()
	if len(fdset.File) == 0 {
		t.Fatal("No files in FileDescriptorSet")
	}

	// Find our message
	var msgFile *descriptorpb.FileDescriptorProto
	for _, file := range fdset.File {
		if file.Package != nil && *file.Package == "test.v1" {
			msgFile = file
			break
		}
	}
	if msgFile == nil {
		t.Fatal("Could not find NewWellKnownMessage file")
	}

	// Verify imports
	expectedImports := map[string]bool{
		"google/protobuf/struct.proto":     false,
		"google/protobuf/field_mask.proto": false,
	}
	for _, imp := range msgFile.Dependency {
		if _, ok := expectedImports[imp]; ok {
			expectedImports[imp] = true
		}
	}
	for imp, found := range expectedImports {
		if !found {
			t.Errorf("Expected import %s not found", imp)
		}
	}

	// Verify message structure
	if len(msgFile.MessageType) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgFile.MessageType))
	}

	msg := msgFile.MessageType[0]
	if msg.Name == nil || *msg.Name != "NewWellKnownMessage" {
		t.Error("Message name mismatch")
	}

	// Check field types
	fieldTests := []struct {
		name         string
		expectedType string
	}{
		{"settings", ".google.protobuf.Struct"},
		{"config", ".google.protobuf.Value"},
		{"values", ".google.protobuf.ListValue"},
		{"update_mask", ".google.protobuf.FieldMask"},
		{"name", ""},
	}

	for _, test := range fieldTests {
		field := findFieldByName(msg, test.name)
		if field == nil {
			t.Errorf("Field %s not found", test.name)
			continue
		}

		if test.expectedType != "" {
			if field.TypeName == nil {
				t.Errorf("Field %s: expected type name %s, got nil", test.name, test.expectedType)
			} else if *field.TypeName != test.expectedType {
				t.Errorf("Field %s: expected type name %s, got %s", test.name, test.expectedType, *field.TypeName)
			}
		}
	}

	// Verify through protoreflect API
	fields := md.Fields()
	if fields.Len() != 5 {
		t.Fatalf("Expected 5 fields, got %d", fields.Len())
	}

	// Check specific field types
	settingsField := fields.ByName("settings")
	if settingsField == nil || !strings.Contains(string(settingsField.Message().FullName()), "google.protobuf.Struct") {
		t.Error("settings field should be google.protobuf.Struct")
	}

	configField := fields.ByName("config")
	if configField == nil || !strings.Contains(string(configField.Message().FullName()), "google.protobuf.Value") {
		t.Error("config field should be google.protobuf.Value")
	}

	valuesField := fields.ByName("values")
	if valuesField == nil || !strings.Contains(string(valuesField.Message().FullName()), "google.protobuf.ListValue") {
		t.Error("values field should be google.protobuf.ListValue")
	}

	updateMaskField := fields.ByName("update_mask")
	if updateMaskField == nil || !strings.Contains(string(updateMaskField.Message().FullName()), "google.protobuf.FieldMask") {
		t.Error("update_mask field should be google.protobuf.FieldMask")
	}
}

func TestBuilder_MixedWellKnownTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	// Test with nested struct containing Well-Known Types
	type NestedConfig struct {
		Data *structpb.Struct `json:"data"`
		Name string           `json:"name"`
	}

	type MixedMessage struct {
		ID         string                     `json:"id"`
		Settings   *structpb.Struct           `json:"settings"`
		Metadata   map[string]*structpb.Value `json:"metadata"`
		UpdateMask *fieldmaskpb.FieldMask     `json:"update_mask"`
		Config     *NestedConfig              `json:"config"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(MixedMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Verify the message was built successfully
	if md.Name() != "MixedMessage" {
		t.Errorf("Expected message name MixedMessage, got %s", md.Name())
	}

	// Get the FileDescriptorSet
	fdset := builder.GetFileDescriptorSet()

	// Find our file
	var msgFile *descriptorpb.FileDescriptorProto
	for _, file := range fdset.File {
		if file.Package != nil && *file.Package == "test.v1" {
			msgFile = file
			break
		}
	}
	if msgFile == nil {
		t.Fatal("Could not find test.v1 file")
	}

	// Should have both MixedMessage and NestedConfig
	if len(msgFile.MessageType) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgFile.MessageType))
	}

	// Find MixedMessage
	var mixedMsg *descriptorpb.DescriptorProto
	for _, msg := range msgFile.MessageType {
		if msg.Name != nil && *msg.Name == "MixedMessage" {
			mixedMsg = msg
			break
		}
	}
	if mixedMsg == nil {
		t.Fatal("Could not find MixedMessage")
	}

	// Check metadata field (map with Value type)
	metadataField := findFieldByName(mixedMsg, "metadata")
	if metadataField == nil {
		t.Fatal("metadata field not found")
	}
	if metadataField.Type == nil || *metadataField.Type != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		t.Error("metadata should be a message type")
	}
	if metadataField.Label == nil || *metadataField.Label != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		t.Error("metadata should be repeated (for map)")
	}
}

// Helper function to find field by name
func findFieldByName(msg *descriptorpb.DescriptorProto, name string) *descriptorpb.FieldDescriptorProto {
	for _, field := range msg.Field {
		if field.Name != nil && *field.Name == name {
			return field
		}
	}
	return nil
}
