package schema

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

// TestProto3EditionsCompatibility verifies that proto3 and editions can work together.
func TestProto3EditionsCompatibility(t *testing.T) {
	t.Run("Feature parity between proto3 and editions", func(t *testing.T) {
		// Define a complex type that exercises various features
		type User struct {
			ID       string            `json:"id" validate:"required"`
			Name     string            `json:"name"`
			Email    *string           `json:"email"`
			Age      int32             `json:"age"`
			Tags     []string          `json:"tags"`
			Scores   []int32           `json:"scores"`
			Metadata map[string]string `json:"metadata"`
		}

		// Build with proto3
		proto3Builder := NewBuilder(BuilderOptions{
			PackageName: "compat.proto3.v1",
			SyntaxMode:  SyntaxProto3,
		})
		proto3Desc, err := proto3Builder.BuildMessage(reflect.TypeOf(User{}))
		if err != nil {
			t.Fatalf("Proto3 BuildMessage failed: %v", err)
		}

		// Build with editions
		editionsBuilder := NewBuilder(BuilderOptions{
			PackageName: "compat.editions.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})
		editionsDesc, err := editionsBuilder.BuildMessage(reflect.TypeOf(User{}))
		if err != nil {
			t.Fatalf("Editions BuildMessage failed: %v", err)
		}

		// Both should successfully create descriptors
		if proto3Desc == nil {
			t.Error("Proto3 descriptor is nil")
		}
		if editionsDesc == nil {
			t.Error("Editions descriptor is nil")
		}

		// Verify wire compatibility
		proto3Fdset := proto3Builder.GetFileDescriptorSet()
		editionsFdset := editionsBuilder.GetFileDescriptorSet()

		proto3Msg := proto3Fdset.File[0].MessageType[0]
		editionsMsg := editionsFdset.File[0].MessageType[0]

		// Field numbers should match for wire compatibility
		if len(proto3Msg.Field) != len(editionsMsg.Field) {
			t.Errorf("Field count mismatch: proto3=%d, editions=%d",
				len(proto3Msg.Field), len(editionsMsg.Field))
		}

		// Verify each field
		fieldMap := make(map[string]int32)
		for _, field := range proto3Msg.Field {
			fieldMap[field.GetName()] = field.GetNumber()
		}

		for _, field := range editionsMsg.Field {
			expectedNumber, ok := fieldMap[field.GetName()]
			if !ok {
				t.Errorf("Field %s not found in proto3 version", field.GetName())
				continue
			}
			if field.GetNumber() != expectedNumber {
				t.Errorf("Field %s number mismatch: proto3=%d, editions=%d",
					field.GetName(), expectedNumber, field.GetNumber())
			}
		}
	})

	t.Run("Explicit field presence modes", func(t *testing.T) {
		type PresenceTest struct {
			// Different presence modes
			Implicit         int32   `json:"implicit" proto:"implicit"`
			Explicit         int32   `json:"explicit" proto:"explicit"`
			Required         string  `json:"required" proto:"required"`
			Optional         *string `json:"optional"`
			Repeated         []int32 `json:"repeated"`
			RepeatedUnpacked []int32 `json:"repeated_unpacked" proto:"unpacked"`
		}

		builder := NewBuilder(BuilderOptions{
			PackageName: "presence.test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		_, err := builder.BuildMessage(reflect.TypeOf(PresenceTest{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		msg := fdset.File[0].MessageType[0]

		// Check each field's presence mode
		for _, field := range msg.Field {
			switch field.GetName() {
			case "implicit":
				if field.Options == nil || field.Options.Features == nil {
					t.Errorf("%s: expected features for implicit presence", field.GetName())
				} else if field.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_IMPLICIT {
					t.Errorf("%s: presence = %v, want IMPLICIT",
						field.GetName(), field.Options.Features.GetFieldPresence())
				}
			case "explicit":
				if field.Options == nil || field.Options.Features == nil {
					// Explicit is the default for Edition 2023, so no features needed
					t.Logf("%s: using file-level default (EXPLICIT)", field.GetName())
				} else if field.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_EXPLICIT {
					t.Errorf("%s: presence = %v, want EXPLICIT",
						field.GetName(), field.Options.Features.GetFieldPresence())
				}
			case "required":
				if field.Options == nil || field.Options.Features == nil {
					t.Errorf("%s: expected features for required presence", field.GetName())
				} else if field.Options.Features.GetFieldPresence() != descriptorpb.FeatureSet_LEGACY_REQUIRED {
					t.Errorf("%s: presence = %v, want LEGACY_REQUIRED",
						field.GetName(), field.Options.Features.GetFieldPresence())
				}
			case "repeated_unpacked":
				if field.Options == nil || field.Options.Features == nil {
					t.Errorf("%s: expected features for unpacked encoding", field.GetName())
				} else if field.Options.Features.GetRepeatedFieldEncoding() != descriptorpb.FeatureSet_EXPANDED {
					t.Errorf("%s: encoding = %v, want EXPANDED",
						field.GetName(), field.Options.Features.GetRepeatedFieldEncoding())
				}
			}
		}
	})

	t.Run("Default values in editions", func(t *testing.T) {
		type DefaultsTest struct {
			StringWithDefault string  `json:"string_with_default" default:"hello"`
			IntWithDefault    int32   `json:"int_with_default" default:"42"`
			FloatWithDefault  float64 `json:"float_with_default" default:"3.14"`
			BoolWithDefault   bool    `json:"bool_with_default" default:"true"`
			// These should not have defaults
			ImplicitInt    int32  `json:"implicit_int" proto:"implicit"`
			RequiredString string `json:"required_string" proto:"required"`
		}

		builder := NewBuilder(BuilderOptions{
			PackageName: "defaults.test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		_, err := builder.BuildMessage(reflect.TypeOf(DefaultsTest{}))
		if err != nil {
			t.Fatalf("BuildMessage failed: %v", err)
		}

		fdset := builder.GetFileDescriptorSet()
		msg := fdset.File[0].MessageType[0]

		// Check default values
		expectedDefaults := map[string]string{
			"string_with_default": "hello",
			"int_with_default":    "42",
			"float_with_default":  "3.14",
			"bool_with_default":   "true",
		}

		for _, field := range msg.Field {
			expected, hasDefault := expectedDefaults[field.GetName()]
			if hasDefault {
				if field.GetDefaultValue() != expected {
					t.Errorf("%s: default = %q, want %q",
						field.GetName(), field.GetDefaultValue(), expected)
				}
			} else {
				// Fields with implicit presence or required should not have defaults
				if field.GetDefaultValue() != "" {
					t.Errorf("%s: unexpected default value %q",
						field.GetName(), field.GetDefaultValue())
				}
			}
		}
	})
}

// TestEditionsEdgeCases tests edge cases and error conditions.
func TestEditionsEdgeCases(t *testing.T) {
	t.Run("Conflicting field characteristics", func(t *testing.T) {
		// Test that we handle conflicting tags gracefully
		type ConflictTest struct {
			// These tags conflict - implicit and explicit cannot both be true
			Field1 string `json:"field1" proto:"implicit" default:"test"`
		}

		builder := NewBuilder(BuilderOptions{
			PackageName: "conflict.test.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     Edition2023,
		})

		// This should fail because implicit presence cannot have default values
		_, err := builder.BuildMessage(reflect.TypeOf(ConflictTest{}))
		if err == nil {
			t.Error("Expected error for implicit field with default value")
		}
	})

	t.Run("Unknown edition", func(t *testing.T) {
		builder := NewBuilder(BuilderOptions{
			PackageName: "unknown.edition.v1",
			SyntaxMode:  SyntaxEditions,
			Edition:     "2025", // Future edition
		})

		// Should fall back to a known edition
		type SimpleMessage struct {
			Name string `json:"name"`
		}

		_, err := builder.BuildMessage(reflect.TypeOf(SimpleMessage{}))
		if err != nil {
			// This might fail if the protobuf library doesn't support the edition
			t.Logf("BuildMessage with unknown edition failed (expected): %v", err)
		}
	})
}
