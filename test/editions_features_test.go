package test

import (
	"strings"
	"testing"

	"github.com/i2y/hyperway/rpc"
	"github.com/i2y/hyperway/schema"
)

func TestEditionsWithFeatures(t *testing.T) {
	// Test struct with various field types to demonstrate editions features
	type EditionsFeatureExample struct {
		// Regular field (implicit presence in proto3, explicit in editions)
		RequiredField string `json:"required_field"`

		// Optional field (should have explicit presence in editions)
		OptionalField *string `json:"optional_field"`

		// Repeated field (packed by default in editions)
		RepeatedInts []int32 `json:"repeated_ints"`

		// Map field
		Metadata map[string]string `json:"metadata"`

		// Bytes field (for UTF-8 validation testing)
		Data []byte `json:"data"`

		// String field (enums via struct tags not yet implemented)
		Status string `json:"status"`
	}

	// Create service with Edition 2023 and custom features
	svc := rpc.NewService("EditionsFeaturesService",
		rpc.WithPackage("editions.features"),
		rpc.WithEdition("2023"),
		// Note: Features are set at the schema level, not service level
		// The default Edition 2023 features will be applied
	)

	// Register a method
	err := svc.Register(&rpc.Method{
		Name: "ProcessData",
		Handler: func(ctx any, req *EditionsFeatureExample) (*EditionsFeatureExample, error) {
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
		if strings.Contains(content, "message EditionsFeatureExample") {
			protoContent = content
			t.Logf("Exported proto file %s:\n%s", filename, content)
			break
		}
	}

	if protoContent == "" {
		t.Fatal("Could not find EditionsFeatureExample message in exported protos")
	}

	// Verify editions syntax
	if !strings.Contains(protoContent, `edition = "2023";`) {
		t.Errorf("Expected proto to contain 'edition = \"2023\";'")
	}

	// Check that optional field is properly marked
	// In editions, optional fields should have explicit field presence
	if !strings.Contains(protoContent, "optional_field") {
		t.Errorf("Expected proto to contain optional_field")
	}

	// Check that all fields are present
	expectedFields := []string{
		"required_field",
		"optional_field",
		"repeated_ints",
		"metadata",
		"data",
		"status",
	}
	for _, field := range expectedFields {
		if !strings.Contains(protoContent, field) {
			t.Errorf("Expected proto to contain field: %s", field)
		}
	}

	// Verify service definition
	if !strings.Contains(protoContent, "service EditionsFeaturesService") {
		t.Errorf("Expected proto to contain service definition")
	}
}

func TestEditionsFeatureDefaults(t *testing.T) {
	// Test that default features are correctly applied for Edition 2023
	features := schema.DefaultEdition2023Features()

	if features == nil {
		t.Fatal("DefaultEdition2023Features returned nil")
	}

	// Edition 2023 defaults to explicit field presence (different from proto3)
	if features.FieldPresence != schema.FieldPresenceExplicit {
		t.Errorf("Expected Edition 2023 to have explicit field presence, got %v", features.FieldPresence)
	}

	// Repeated fields should be packed by default
	if features.RepeatedFieldEncoding != schema.RepeatedFieldEncodingPacked {
		t.Errorf("Expected Edition 2023 to have packed repeated field encoding, got %v", features.RepeatedFieldEncoding)
	}

	// Enums should be open by default
	if features.EnumType != schema.EnumTypeOpen {
		t.Errorf("Expected Edition 2023 to have open enum type, got %v", features.EnumType)
	}

	// UTF-8 validation should be enabled
	if features.UTF8Validation != schema.UTF8ValidationVerify {
		t.Errorf("Expected Edition 2023 to have UTF-8 validation enabled, got %v", features.UTF8Validation)
	}
}

func TestMultipleEditionsServices(t *testing.T) {
	// Test that we can have services with different editions in the same application

	// Proto3 service
	proto3Svc := rpc.NewService("Proto3Service",
		rpc.WithPackage("multi.proto3"),
		// No edition specified, defaults to proto3
	)

	// Edition 2023 service
	editionsSvc := rpc.NewService("EditionsService",
		rpc.WithPackage("multi.editions"),
		rpc.WithEdition("2023"),
	)

	type SimpleMessage struct {
		Value string `json:"value"`
	}

	// Register methods
	err := proto3Svc.Register(&rpc.Method{
		Name: "Echo",
		Handler: func(ctx any, req *SimpleMessage) (*SimpleMessage, error) {
			return req, nil
		},
	})
	if err != nil {
		t.Fatalf("Proto3 register failed: %v", err)
	}

	err = editionsSvc.Register(&rpc.Method{
		Name: "Echo",
		Handler: func(ctx any, req *SimpleMessage) (*SimpleMessage, error) {
			return req, nil
		},
	})
	if err != nil {
		t.Fatalf("Editions register failed: %v", err)
	}

	// Export and verify both
	proto3Protos, err := proto3Svc.ExportAllProtos()
	if err != nil {
		t.Fatalf("Proto3 export failed: %v", err)
	}

	editionsProtos, err := editionsSvc.ExportAllProtos()
	if err != nil {
		t.Fatalf("Editions export failed: %v", err)
	}

	// Verify proto3 syntax
	for _, content := range proto3Protos {
		if strings.Contains(content, "service Proto3Service") {
			if !strings.Contains(content, `syntax = "proto3";`) {
				t.Errorf("Proto3 service should have 'syntax = \"proto3\";'")
			}
			break
		}
	}

	// Verify editions syntax
	for _, content := range editionsProtos {
		if strings.Contains(content, "service EditionsService") {
			if !strings.Contains(content, `edition = "2023";`) {
				t.Errorf("Editions service should have 'edition = \"2023\";'")
			}
			break
		}
	}
}
