package schema

import (
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestStringToEdition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected descriptorpb.Edition
	}{
		{
			name:     "Edition 2023 string",
			input:    "2023",
			expected: descriptorpb.Edition_EDITION_2023,
		},
		{
			name:     "Edition 2024 string",
			input:    "2024",
			expected: descriptorpb.Edition_EDITION_2024,
		},
		{
			name:     "EDITION_2023 format",
			input:    "EDITION_2023",
			expected: descriptorpb.Edition_EDITION_2023,
		},
		{
			name:     "EDITION_2024 format",
			input:    "EDITION_2024",
			expected: descriptorpb.Edition_EDITION_2024,
		},
		{
			name:     "Unknown edition defaults to 2023",
			input:    "unknown",
			expected: descriptorpb.Edition_EDITION_2023,
		},
		{
			name:     "Empty string defaults to 2023",
			input:    "",
			expected: descriptorpb.Edition_EDITION_2023,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToEdition(tt.input)
			if result == nil {
				t.Fatal("StringToEdition returned nil")
			}
			if *result != tt.expected {
				t.Errorf("StringToEdition(%q) = %v, want %v", tt.input, *result, tt.expected)
			}
		})
	}
}

func TestDefaultProto3Features(t *testing.T) {
	features := DefaultProto3Features()

	if features == nil {
		t.Fatal("DefaultProto3Features returned nil")
	}

	if features.FieldPresence != FieldPresenceImplicit {
		t.Errorf("Proto3 field presence = %v, want %v", features.FieldPresence, FieldPresenceImplicit)
	}

	if features.RepeatedFieldEncoding != RepeatedFieldEncodingPacked {
		t.Errorf("Proto3 repeated field encoding = %v, want %v", features.RepeatedFieldEncoding, RepeatedFieldEncodingPacked)
	}

	if features.EnumType != EnumTypeOpen {
		t.Errorf("Proto3 enum type = %v, want %v", features.EnumType, EnumTypeOpen)
	}

	if features.UTF8Validation != UTF8ValidationVerify {
		t.Errorf("Proto3 UTF8 validation = %v, want %v", features.UTF8Validation, UTF8ValidationVerify)
	}
}

func TestDefaultEdition2023Features(t *testing.T) {
	features := DefaultEdition2023Features()

	if features == nil {
		t.Fatal("DefaultEdition2023Features returned nil")
	}

	if features.FieldPresence != FieldPresenceExplicit {
		t.Errorf("Edition 2023 field presence = %v, want %v", features.FieldPresence, FieldPresenceExplicit)
	}

	if features.RepeatedFieldEncoding != RepeatedFieldEncodingPacked {
		t.Errorf("Edition 2023 repeated field encoding = %v, want %v", features.RepeatedFieldEncoding, RepeatedFieldEncodingPacked)
	}

	if features.EnumType != EnumTypeOpen {
		t.Errorf("Edition 2023 enum type = %v, want %v", features.EnumType, EnumTypeOpen)
	}

	if features.UTF8Validation != UTF8ValidationVerify {
		t.Errorf("Edition 2023 UTF8 validation = %v, want %v", features.UTF8Validation, UTF8ValidationVerify)
	}
}

func TestFeatureSetClone(t *testing.T) {
	original := &FeatureSet{
		FieldPresence:         FieldPresenceExplicit,
		RepeatedFieldEncoding: RepeatedFieldEncodingExpanded,
		EnumType:              EnumTypeClosed,
		UTF8Validation:        UTF8ValidationNone,
	}

	clone := original.Clone()

	if clone == nil {
		t.Fatal("Clone returned nil")
	}

	if clone == original {
		t.Error("Clone returned same pointer as original")
	}

	if clone.FieldPresence != original.FieldPresence {
		t.Errorf("Clone field presence = %v, want %v", clone.FieldPresence, original.FieldPresence)
	}

	if clone.RepeatedFieldEncoding != original.RepeatedFieldEncoding {
		t.Errorf("Clone repeated field encoding = %v, want %v", clone.RepeatedFieldEncoding, original.RepeatedFieldEncoding)
	}

	if clone.EnumType != original.EnumType {
		t.Errorf("Clone enum type = %v, want %v", clone.EnumType, original.EnumType)
	}

	if clone.UTF8Validation != original.UTF8Validation {
		t.Errorf("Clone UTF8 validation = %v, want %v", clone.UTF8Validation, original.UTF8Validation)
	}

	// Test nil clone
	var nilFeatures *FeatureSet
	nilClone := nilFeatures.Clone()
	if nilClone != nil {
		t.Error("Clone of nil FeatureSet should return nil")
	}
}
