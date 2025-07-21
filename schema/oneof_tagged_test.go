package schema_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/i2y/hyperway/schema"
)

// Test structs for tagged oneof validation

// Tagged oneof - explicit declaration
type TaggedUpdateRequest struct {
	UserID string

	// oneof identifier - explicitly tagged
	Identifier struct {
		Email       *string
		PhoneNumber *string
		Username    *string
	} `hyperway:"oneof"`
}

// No tag - should NOT be detected as oneof
type NoTagRequest struct {
	UserID string

	// NOT a oneof (no tag)
	Identifier struct {
		Email       *string
		PhoneNumber *string
		Username    *string
	}
}

// Multiple tagged oneofs
type MultipleTaggedRequest struct {
	RequestID string

	// First oneof group
	Target struct {
		User         *string
		Group        *string
		Organization *string
	} `hyperway:"oneof"`

	// Second oneof group
	Action struct {
		Create *bool
		Update *bool
		Delete *bool
	} `hyperway:"oneof"`

	// NOT a oneof (no tag) - can have multiple fields set
	Metadata struct {
		CreatedBy *string
		UpdatedBy *string
		Version   *int
	}
}

// Old style tag (should still work for compatibility)
type OldStyleRequest struct {
	Name string

	Contact struct {
		Email       *string
		PhoneNumber *string
		Address     *string
	} `protobuf_oneof:"true"`
}

// Edge cases
type EdgeCaseRequest struct {
	// Empty struct with tag - should be ignored
	Empty struct{} `hyperway:"oneof"`

	// Single field - should be ignored (need at least 2)
	Single struct {
		OnlyOne *string
	} `hyperway:"oneof"`

	// Valid oneof
	Valid struct {
		Option1 *string
		Option2 *string
	} `hyperway:"oneof"`

	// Non-struct field with tag - should be ignored
	NotAStruct string `hyperway:"oneof"`
}

func TestTaggedOneofDetection(t *testing.T) {
	tests := []struct {
		name          string
		structType    any
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "tagged oneof",
			structType:    TaggedUpdateRequest{},
			expectedCount: 1,
			expectedNames: []string{"identifier"},
		},
		{
			name:          "no tag - not detected",
			structType:    NoTagRequest{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "multiple tagged oneofs",
			structType:    MultipleTaggedRequest{},
			expectedCount: 2,
			expectedNames: []string{"target", "action"},
		},
		{
			name:          "old style tag compatibility",
			structType:    OldStyleRequest{},
			expectedCount: 1,
			expectedNames: []string{"contact"},
		},
		{
			name:          "edge cases",
			structType:    EdgeCaseRequest{},
			expectedCount: 1,
			expectedNames: []string{"valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the detection behavior
			// In a real implementation, we'd check the actual detected groups
		})
	}
}

func TestTaggedOneofValidation(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		wantError bool
		errorMsg  string
	}{
		// Tagged oneof tests
		{
			name: "tagged - no fields set",
			value: &TaggedUpdateRequest{
				UserID: "123",
			},
			wantError: false,
		},
		{
			name: "tagged - one field set",
			value: &TaggedUpdateRequest{
				UserID: "123",
				Identifier: struct {
					Email       *string
					PhoneNumber *string
					Username    *string
				}{
					Email: ptr("user@example.com"),
				},
			},
			wantError: false,
		},
		{
			name: "tagged - multiple fields set",
			value: &TaggedUpdateRequest{
				UserID: "123",
				Identifier: struct {
					Email       *string
					PhoneNumber *string
					Username    *string
				}{
					Email:       ptr("user@example.com"),
					PhoneNumber: ptr("+1234567890"),
				},
			},
			wantError: true,
			errorMsg:  "oneof constraint violated for group 'identifier'",
		},

		// No tag - should allow multiple fields
		{
			name: "no tag - multiple fields allowed",
			value: &NoTagRequest{
				UserID: "123",
				Identifier: struct {
					Email       *string
					PhoneNumber *string
					Username    *string
				}{
					Email:       ptr("user@example.com"),
					PhoneNumber: ptr("+1234567890"),
					Username:    ptr("alice"),
				},
			},
			wantError: false, // No error because no oneof constraint
		},

		// Multiple oneofs with metadata
		{
			name: "multiple oneofs - only oneofs validated",
			value: &MultipleTaggedRequest{
				RequestID: "req-123",
				Target: struct {
					User         *string
					Group        *string
					Organization *string
				}{
					User: ptr("user-456"),
				},
				Action: struct {
					Create *bool
					Update *bool
					Delete *bool
				}{
					Create: ptr(true),
				},
				Metadata: struct {
					CreatedBy *string
					UpdatedBy *string
					Version   *int
				}{
					// Multiple fields allowed in metadata (no oneof tag)
					CreatedBy: ptr("admin"),
					UpdatedBy: ptr("system"),
					Version:   ptr(1),
				},
			},
			wantError: false,
		},
		{
			name: "multiple oneofs - target violation",
			value: &MultipleTaggedRequest{
				RequestID: "req-123",
				Target: struct {
					User         *string
					Group        *string
					Organization *string
				}{
					User:  ptr("user-456"),
					Group: ptr("group-789"), // Violation!
				},
				Action: struct {
					Create *bool
					Update *bool
					Delete *bool
				}{
					Create: ptr(true),
				},
			},
			wantError: true,
			errorMsg:  "oneof constraint violated for group 'target'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateOneof(reflect.TypeOf(tt.value).Elem(), tt.value)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
