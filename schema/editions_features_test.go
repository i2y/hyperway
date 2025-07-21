package schema

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEditionsFeatures(t *testing.T) {
	t.Run("File-level features for Edition 2023", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		type TestMessage struct {
			Name string `json:"name"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		if len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		file := fdset.File[0]

		// Check that file has editions syntax
		if file.GetSyntax() != "editions" {
			t.Errorf("File syntax = %q, want %q", file.GetSyntax(), "editions")
		}

		// Check that file has edition set
		if file.Edition == nil || *file.Edition != descriptorpb.Edition_EDITION_2023 {
			t.Error("File should have Edition 2023 set")
		}

		// Check that file has features set
		if file.Options == nil || file.Options.Features == nil {
			t.Fatal("File should have features set in options")
		}

		features := file.Options.Features

		// Verify Edition 2023 defaults
		if features.GetFieldPresence() != descriptorpb.FeatureSet_EXPLICIT {
			t.Errorf("Field presence = %v, want EXPLICIT", features.GetFieldPresence())
		}

		if features.GetEnumType() != descriptorpb.FeatureSet_OPEN {
			t.Errorf("Enum type = %v, want OPEN", features.GetEnumType())
		}

		if features.GetRepeatedFieldEncoding() != descriptorpb.FeatureSet_PACKED {
			t.Errorf("Repeated field encoding = %v, want PACKED", features.GetRepeatedFieldEncoding())
		}

		if features.GetUtf8Validation() != descriptorpb.FeatureSet_VERIFY {
			t.Errorf("UTF8 validation = %v, want VERIFY", features.GetUtf8Validation())
		}
	})

	t.Run("Field-level features override", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		type TestMessage struct {
			// Regular field - should use file defaults
			Name string `json:"name"`
			// Field with implicit presence
			Count int32 `json:"count" proto:"implicit"`
			// Required field
			ID string `json:"id" proto:"required"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		file := fdset.File[0]
		msg := file.MessageType[0]

		// Check Name field - should use file defaults (no field-level features)
		var nameField *descriptorpb.FieldDescriptorProto
		for _, field := range msg.Field {
			if field.GetName() == "name" {
				nameField = field
				break
			}
		}
		if nameField == nil {
			t.Fatal("Name field not found")
		}
		if nameField.Options != nil && nameField.Options.Features != nil {
			t.Error("Name field should not have field-level features (uses file defaults)")
		}

		// Check Count field - should have implicit presence
		var countField *descriptorpb.FieldDescriptorProto
		for _, field := range msg.Field {
			if field.GetName() == "count" {
				countField = field
				break
			}
		}
		if countField == nil {
			t.Fatal("Count field not found")
		}
		if countField.Options == nil || countField.Options.Features == nil {
			t.Fatal("Count field should have field-level features")
		}
		if countField.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_IMPLICIT {
			t.Errorf("Count field presence = %v, want IMPLICIT", countField.Options.Features.GetFieldPresence())
		}

		// Check ID field - should have required presence
		var idField *descriptorpb.FieldDescriptorProto
		for _, field := range msg.Field {
			if field.GetName() == "id" {
				idField = field
				break
			}
		}
		if idField == nil {
			t.Fatal("ID field not found")
		}
		if idField.Options == nil || idField.Options.Features == nil {
			t.Fatal("ID field should have field-level features")
		}
		if idField.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_LEGACY_REQUIRED {
			t.Errorf("ID field presence = %v, want LEGACY_REQUIRED", idField.Options.Features.GetFieldPresence())
		}
	})

	t.Run("Default values in Editions", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		type TestMessage struct {
			Name  string `json:"name" default:"Unknown"`
			Count int32  `json:"count" default:"42"`
			Flag  bool   `json:"flag" default:"true"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		file := fdset.File[0]
		msg := file.MessageType[0]

		// Check default values
		for _, field := range msg.Field {
			switch field.GetName() {
			case "name":
				if field.GetDefaultValue() != "Unknown" {
					t.Errorf("Name field default = %q, want %q", field.GetDefaultValue(), "Unknown")
				}
			case "count":
				if field.GetDefaultValue() != "42" {
					t.Errorf("Count field default = %q, want %q", field.GetDefaultValue(), "42")
				}
			case "flag":
				if field.GetDefaultValue() != "true" {
					t.Errorf("Flag field default = %q, want %q", field.GetDefaultValue(), "true")
				}
			}
		}
	})

	t.Run("Proto3 vs Editions field presence", func(t *testing.T) {
		// Proto3 test
		proto3Builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxProto3,
		})

		type TestMessage struct {
			Name     string  `json:"name"`     // Implicit presence in proto3
			Optional *string `json:"optional"` // Explicit presence via proto3_optional
		}

		_, err := proto3Builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("Proto3 BuildMessage failed: %v", err)
		}

		proto3Fdset := proto3Builder.GetFileDescriptorSet()
		proto3File := proto3Fdset.File[0]
		proto3Msg := proto3File.MessageType[0]

		// Check proto3 optional field
		var proto3OptionalField *descriptorpb.FieldDescriptorProto
		for _, field := range proto3Msg.Field {
			if field.GetName() == "optional" {
				proto3OptionalField = field
				break
			}
		}
		if !proto3OptionalField.GetProto3Optional() {
			t.Error("Proto3 pointer field should have proto3_optional set")
		}

		// Editions test
		editionsBuilder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		_, err = editionsBuilder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("Editions BuildMessage failed: %v", err)
		}

		editionsFdset := editionsBuilder.GetFileDescriptorSet()
		editionsFile := editionsFdset.File[0]
		editionsMsg := editionsFile.MessageType[0]

		// Check editions optional field
		var editionsOptionalField *descriptorpb.FieldDescriptorProto
		for _, field := range editionsMsg.Field {
			if field.GetName() == "optional" {
				editionsOptionalField = field
				break
			}
		}
		// In Editions, proto3_optional should NOT be set
		if editionsOptionalField.GetProto3Optional() {
			t.Error("Editions field should not have proto3_optional set")
		}
		// Instead, presence is controlled by features (default is EXPLICIT in Edition 2023)
	})
}

func TestFeaturesInheritance(t *testing.T) {
	t.Run("Message-level features", func(t *testing.T) {
		// This test would require implementing message-level features
		// Currently not implemented, so we skip this test
		t.Skip("Message-level features not yet implemented")
	})
}
