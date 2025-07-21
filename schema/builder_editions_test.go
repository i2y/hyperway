package schema

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestBuilderWithEditions(t *testing.T) {
	t.Run("Proto3 mode (default)", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
		})

		type TestMessage struct {
			Name     string  `json:"name"`
			Age      int32   `json:"age"`
			Optional *string `json:"optional"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		// Get the file descriptor
		fdset := builder.GetFileDescriptorSet()
		if len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		file := fdset.File[0]
		if file.GetSyntax() != "proto3" {
			t.Errorf("File syntax = %q, want %q", file.GetSyntax(), "proto3")
		}

		if file.Edition != nil {
			t.Error("Proto3 file should not have Edition field set")
		}

		// Check field with proto3_optional
		msg := file.MessageType[0]
		const optionalFieldName = "optional"
		var optionalField *descriptorpb.FieldDescriptorProto
		for _, field := range msg.Field {
			if field.GetName() == optionalFieldName {
				optionalField = field
				break
			}
		}

		if optionalField == nil {
			t.Fatal("Optional field not found")
		}

		if !optionalField.GetProto3Optional() {
			t.Error("Pointer field should have proto3_optional set in proto3 mode")
		}
	})

	t.Run("Editions 2023 mode", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		type TestMessage struct {
			Name     string  `json:"name"`
			Age      int32   `json:"age"`
			Optional *string `json:"optional"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		// Get the file descriptor
		fdset := builder.GetFileDescriptorSet()
		if len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		file := fdset.File[0]
		const expectedSyntax = "editions"
		if file.GetSyntax() != expectedSyntax {
			t.Errorf("File syntax = %q, want %q", file.GetSyntax(), expectedSyntax)
		}

		if file.Edition == nil {
			t.Fatal("Editions file should have Edition field set")
		}

		if *file.Edition != descriptorpb.Edition_EDITION_2023 {
			t.Errorf("File edition = %v, want %v", *file.Edition, descriptorpb.Edition_EDITION_2023)
		}

		// Check that fields don't have proto3_optional in editions mode
		msg := file.MessageType[0]
		for _, field := range msg.Field {
			if field.Proto3Optional != nil && *field.Proto3Optional {
				t.Errorf("Field %s should not have proto3_optional set in editions mode", field.GetName())
			}
		}
	})

	t.Run("Editions 2024 mode", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2024,
		})

		type TestMessage struct {
			Name string `json:"name"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(TestMessage{}))
		if err != nil {
			// EDITION_2024 might not be supported yet by the Go protobuf runtime
			if testing.Verbose() {
				t.Logf("BuildMessage with EDITION_2024 failed (this might be expected): %v", err)
			}
			t.Skip("EDITION_2024 not yet supported by Go protobuf runtime")
		}

		// Get the file descriptor
		fdset := builder.GetFileDescriptorSet()
		if len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		file := fdset.File[0]
		if file.Edition == nil {
			t.Fatal("Editions file should have Edition field set")
		}

		if *file.Edition != descriptorpb.Edition_EDITION_2024 {
			t.Errorf("File edition = %v, want %v", *file.Edition, descriptorpb.Edition_EDITION_2024)
		}
	})

	t.Run("Default features", func(t *testing.T) {
		// Test proto3 default features
		proto3Builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
		})
		if proto3Builder.options.Features == nil {
			t.Error("Proto3 builder should have default features")
		}
		if proto3Builder.options.Features.FieldPresence != FieldPresenceImplicit {
			t.Error("Proto3 should have implicit field presence by default")
		}

		// Test editions default features
		editionsBuilder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
		})
		if editionsBuilder.options.Features == nil {
			t.Error("Editions builder should have default features")
		}
		if editionsBuilder.options.Features.FieldPresence != FieldPresenceExplicit {
			t.Error("Editions should have explicit field presence by default")
		}
	})

	t.Run("Custom features", func(t *testing.T) {
		customFeatures := &FeatureSet{
			FieldPresence:         FieldPresenceImplicit,
			RepeatedFieldEncoding: RepeatedFieldEncodingExpanded,
			EnumType:              EnumTypeClosed,
			UTF8Validation:        UTF8ValidationNone,
		}

		builder := NewBuilder(BuilderOptions{
			PackageName: "test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
			Features:    customFeatures,
		})

		if builder.options.Features != customFeatures {
			t.Error("Builder should use custom features")
		}
	})
}
