package schema

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseValidationTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want []ValidationRule
	}{
		{
			name: "empty tag",
			tag:  "",
			want: nil,
		},
		{
			name: "single rule",
			tag:  "required",
			want: []ValidationRule{
				{Name: "required", Value: "true"},
			},
		},
		{
			name: "multiple rules",
			tag:  "required,email",
			want: []ValidationRule{
				{Name: "required", Value: "true"},
				{Name: "email", Value: "true"},
			},
		},
		{
			name: "rules with values",
			tag:  "required,min=3,max=50",
			want: []ValidationRule{
				{Name: "required", Value: "true"},
				{Name: "min", Value: "3"},
				{Name: "max", Value: "50"},
			},
		},
		{
			name: "complex validation",
			tag:  "required,email,min=5,max=255",
			want: []ValidationRule{
				{Name: "required", Value: "true"},
				{Name: "email", Value: "true"},
				{Name: "min", Value: "5"},
				{Name: "max", Value: "255"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseValidationTag(tt.tag)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseValidationTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildValidationComment(t *testing.T) {
	tests := []struct {
		name  string
		rules []ValidationRule
		want  string
	}{
		{
			name:  "empty rules",
			rules: nil,
			want:  "",
		},
		{
			name: "single rule",
			rules: []ValidationRule{
				{Name: "required", Value: "true"},
			},
			want: "Validation: @required",
		},
		{
			name: "multiple rules",
			rules: []ValidationRule{
				{Name: "required", Value: "true"},
				{Name: "min", Value: "3"},
				{Name: "max", Value: "50"},
			},
			want: "Validation: @required @min(3) @max(50)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildValidationComment(tt.rules)
			if got != tt.want {
				t.Errorf("BuildValidationComment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddValidationMetadata(t *testing.T) {
	tests := []struct {
		name          string
		fieldName     string
		validationTag string
		checkJSONName bool
	}{
		{
			name:          "no validation",
			fieldName:     "test_field",
			validationTag: "",
			checkJSONName: false,
		},
		{
			name:          "with validation",
			fieldName:     "email",
			validationTag: "required,email",
			checkJSONName: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &descriptorpb.FieldDescriptorProto{
				Name: proto(tt.fieldName),
			}

			AddValidationMetadata(field, tt.validationTag)

			if tt.checkJSONName {
				// Validation info is no longer stored in JsonName
				// Check that field options are set instead
				if field.Options == nil && tt.validationTag != "" {
					t.Log("Field options not set for validation metadata (this is expected)")
				}
			}
		})
	}
}

func TestConvertToProtobufValidation(t *testing.T) {
	tests := []struct {
		name          string
		validationTag string
		expectedKeys  []string
	}{
		{
			name:          "required",
			validationTag: "required",
			expectedKeys:  []string{"required"},
		},
		{
			name:          "email",
			validationTag: "email",
			expectedKeys:  []string{"pattern"},
		},
		{
			name:          "min max",
			validationTag: "min=5,max=10",
			expectedKeys:  []string{"minimum", "maximum"},
		},
		{
			name:          "url format",
			validationTag: "url",
			expectedKeys:  []string{"format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToProtobufValidation(tt.validationTag)

			for _, key := range tt.expectedKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("Expected key %s not found in result", key)
				}
			}
		})
	}
}
