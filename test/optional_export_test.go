package test

import (
	"strings"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

func TestOptionalFieldsProtoExport(t *testing.T) {
	type OptionalExample struct {
		// Regular fields (no optional keyword in proto3)
		RegularString string `json:"regular_string"`
		RegularInt    int32  `json:"regular_int"`

		// Pointer fields (should have optional keyword)
		OptionalString *string `json:"optional_string"`
		OptionalInt    *int32  `json:"optional_int"`
		OptionalBool   *bool   `json:"optional_bool"`

		// Explicit optional via tag
		TaggedOptional string `json:"tagged_optional" proto:"optional"`
	}

	svc := rpc.NewService("TestService", rpc.WithPackage("test"))

	// Register a dummy method to ensure the message is included
	err := svc.Register(&rpc.Method{
		Name: "TestMethod",
		Handler: func(ctx any, req *OptionalExample) (*OptionalExample, error) {
			return req, nil
		},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Export all proto files
	allProtos, err := svc.ExportAllProtos()
	if err != nil {
		t.Fatalf("ExportAllProtos failed: %v", err)
	}

	// Find the message proto file
	var protoContent string
	for filename, content := range allProtos {
		t.Logf("File %s:\n%s", filename, content)
		if strings.Contains(content, "message OptionalExample") {
			protoContent = content
			break
		}
	}

	if protoContent == "" {
		t.Fatal("Could not find OptionalExample message in exported protos")
	}

	// Check for optional keywords
	optionalFields := []string{
		"optional string optional_string",
		"optional int32 optional_int",
		"optional bool optional_bool",
		"optional string tagged_optional",
	}

	for _, expected := range optionalFields {
		if !strings.Contains(protoContent, expected) {
			t.Errorf("Expected proto to contain '%s'", expected)
		}
	}

	// Check that regular fields don't have optional keyword
	regularFields := []string{
		"string regular_string",
		"int32 regular_int",
	}

	for _, field := range regularFields {
		// Should not have "optional" before the field
		if strings.Contains(protoContent, "optional "+field) {
			t.Errorf("Field '%s' should not have optional keyword", field)
		}
		// But should still exist
		if !strings.Contains(protoContent, field) {
			t.Errorf("Field '%s' not found in proto", field)
		}
	}
}
