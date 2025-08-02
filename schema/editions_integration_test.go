package schema

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEditionsIntegration(t *testing.T) { //nolint:gocyclo // This is a comprehensive test suite
	t.Run("Editions with custom features and defaults", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.editions.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		type Order struct {
			ID          string   `json:"id" proto:"required"`
			Description string   `json:"description" default:"No description"`
			Quantity    int32    `json:"quantity" proto:"implicit"` // Cannot have default with implicit presence
			Price       float64  `json:"price" default:"0.0"`
			IsPaid      bool     `json:"is_paid" default:"false"`
			Tags        []string `json:"tags" proto:"unpacked"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(Order{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		file := fdset.File[0]
		msg := file.MessageType[0]

		// Map to store fields by name for easier access
		fields := make(map[string]*descriptorpb.FieldDescriptorProto)
		for _, field := range msg.Field {
			fields[field.GetName()] = field
		}

		// Test ID field (required)
		idField := fields["id"]
		if idField == nil {
			t.Fatal("ID field not found")
		}
		if idField.Options == nil || idField.Options.Features == nil {
			t.Error("ID field should have features for required presence")
		} else if idField.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_LEGACY_REQUIRED {
			t.Errorf("ID field presence = %v, want LEGACY_REQUIRED", idField.Options.Features.GetFieldPresence())
		}

		// Test Description field (default value)
		descField := fields["description"]
		if descField == nil {
			t.Fatal("Description field not found")
		}
		if descField.GetDefaultValue() != "No description" {
			t.Errorf("Description default = %q, want %q", descField.GetDefaultValue(), "No description")
		}

		// Test Quantity field (implicit presence)
		qtyField := fields["quantity"]
		if qtyField == nil {
			t.Fatal("Quantity field not found")
		}
		if qtyField.Options == nil || qtyField.Options.Features == nil {
			t.Error("Quantity field should have features for implicit presence")
		} else if qtyField.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_IMPLICIT {
			t.Errorf("Quantity field presence = %v, want IMPLICIT", qtyField.Options.Features.GetFieldPresence())
		}
		// Note: Cannot have default value with implicit presence

		// Test Price field (default value)
		priceField := fields["price"]
		if priceField == nil {
			t.Fatal("Price field not found")
		}
		if priceField.GetDefaultValue() != "0.0" {
			t.Errorf("Price default = %q, want %q", priceField.GetDefaultValue(), "0.0")
		}

		// Test Tags field (unpacked repeated)
		tagsField := fields["tags"]
		if tagsField == nil {
			t.Fatal("Tags field not found")
		}
		if tagsField.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			t.Error("Tags field should be repeated")
		}
		if tagsField.Options == nil || tagsField.Options.Features == nil {
			t.Error("Tags field should have features for unpacked encoding")
		} else if tagsField.Options.Features.GetRepeatedFieldEncoding() != descriptorpb.FeatureSet_EXPANDED {
			t.Errorf("Tags field encoding = %v, want EXPANDED (unpacked)", tagsField.Options.Features.GetRepeatedFieldEncoding())
		}
	})

	t.Run("Compare proto3 and editions behavior", func(t *testing.T) {
		type TestStruct struct {
			Name    string  `json:"name"`
			Count   int32   `json:"count"`
			Values  []int32 `json:"values"`
			Details *string `json:"details"`
		}

		// Build with proto3
		proto3Builder := NewBuilder(BuilderOptions{
			PackageName: "test.proto3.v1",
			SyntaxMode:  SyntaxProto3,
		})
		_, err := proto3Builder.BuildMessage(reflect.TypeOf(TestStruct{}))
		if err != nil {
			t.Fatalf("Proto3 BuildMessage failed: %v", err)
		}

		// Build with editions
		editionsBuilder := NewBuilder(BuilderOptions{
			PackageName: "test.editions.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})
		_, err = editionsBuilder.BuildMessage(reflect.TypeOf(TestStruct{}))
		if err != nil {
			t.Fatalf("Editions BuildMessage failed: %v", err)
		}

		proto3File := proto3Builder.GetFileDescriptorSet().File[0]
		editionsFile := editionsBuilder.GetFileDescriptorSet().File[0]

		// Verify syntax differences
		if proto3File.GetSyntax() != "proto3" {
			t.Errorf("Proto3 file syntax = %q, want %q", proto3File.GetSyntax(), "proto3")
		}
		if editionsFile.GetSyntax() != "editions" {
			t.Errorf("Editions file syntax = %q, want %q", editionsFile.GetSyntax(), "editions")
		}

		// Verify edition field
		if proto3File.Edition != nil {
			t.Error("Proto3 file should not have edition set")
		}
		if editionsFile.Edition == nil || *editionsFile.Edition != descriptorpb.Edition_EDITION_2023 {
			t.Error("Editions file should have Edition 2023 set")
		}

		// Verify features
		if proto3File.Options != nil && proto3File.Options.Features != nil {
			// Proto3 can have features for compatibility, but it's not required
			t.Log("Proto3 file has features (allowed for compatibility)")
		}
		if editionsFile.Options == nil || editionsFile.Options.Features == nil {
			t.Error("Editions file must have features set")
		}

		// Check field handling differences
		proto3Msg := proto3File.MessageType[0]
		editionsMsg := editionsFile.MessageType[0]

		// Find Details field in both
		var proto3DetailsField, editionsDetailsField *descriptorpb.FieldDescriptorProto
		for _, field := range proto3Msg.Field {
			if field.GetName() == "details" {
				proto3DetailsField = field
				break
			}
		}
		for _, field := range editionsMsg.Field {
			if field.GetName() == "details" {
				editionsDetailsField = field
				break
			}
		}

		// In proto3, pointer fields have proto3_optional
		if !proto3DetailsField.GetProto3Optional() {
			t.Error("Proto3 pointer field should have proto3_optional")
		}

		// In editions, field presence is controlled by features, not proto3_optional
		if editionsDetailsField.GetProto3Optional() {
			t.Error("Editions field should not use proto3_optional")
		}
	})
}
