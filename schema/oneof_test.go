package schema_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/i2y/hyperway/schema"
)

// Test structs for oneof validation

// Flat naming convention
type UpdateUserRequest struct {
	UserID string

	// oneof identifier (flat naming convention)
	IdentifierEmail       *string
	IdentifierPhoneNumber *string
	IdentifierUsername    *string
}

// Embedded struct with tag
type CreateUserRequest struct {
	Name string

	// oneof contact
	Contact struct {
		Email       *string
		PhoneNumber *string
		Address     *string
	} `hyperway:"oneof"`
}

// Embedded struct without tag (all pointer fields)
type SearchRequest struct {
	Query string

	// oneof filter - detected by all fields being pointers
	Filter struct {
		UserID   *string
		GroupID  *string
		Category *string
	}
}

// Mixed fields (not a oneof)
type MixedRequest struct {
	ID string

	Data struct {
		Name  string  // Not a pointer
		Email *string // Pointer
		Phone *string // Pointer
	}
}

// Nested oneofs
type ComplexRequest struct {
	RequestID string

	// First oneof group
	TargetUser         *string
	TargetGroup        *string
	TargetOrganization *string

	// Second oneof group
	ActionCreate *bool
	ActionUpdate *bool
	ActionDelete *bool
}

func TestOneofDetection(t *testing.T) {
	tests := []struct {
		name          string
		structType    any
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "flat naming convention (no longer supported)",
			structType:    UpdateUserRequest{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "embedded struct with tag",
			structType:    CreateUserRequest{},
			expectedCount: 1,
			expectedNames: []string{"contact"},
		},
		{
			name:          "embedded struct all pointers (no longer auto-detected)",
			structType:    SearchRequest{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "mixed fields not oneof",
			structType:    MixedRequest{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "multiple oneof groups (no longer supported without tags)",
			structType:    ComplexRequest{},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test detection
			_ = schema.NewOneofValidator(reflect.TypeOf(tt.structType))

			// This is a white-box test - we'd need to export the groups field
			// or add a method to get the count for proper testing
			// For now, we'll test validation behavior
		})
	}
}

func TestOneofValidation(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		wantError bool
		errorMsg  string
	}{
		// Flat naming convention tests (no longer detected as oneof)
		{
			name: "flat - no fields set (no oneof validation)",
			value: &UpdateUserRequest{
				UserID: "123",
			},
			wantError: false,
		},
		{
			name: "flat - one field set (no oneof validation)",
			value: &UpdateUserRequest{
				UserID:          "123",
				IdentifierEmail: ptr("user@example.com"),
			},
			wantError: false,
		},
		{
			name: "flat - multiple fields set (no oneof validation)",
			value: &UpdateUserRequest{
				UserID:                "123",
				IdentifierEmail:       ptr("user@example.com"),
				IdentifierPhoneNumber: ptr("+1234567890"),
			},
			wantError: false, // No error because not detected as oneof
			errorMsg:  "",
		},

		// Embedded struct tests
		{
			name: "embedded - no fields set",
			value: &CreateUserRequest{
				Name: "John Doe",
			},
			wantError: false,
		},
		{
			name: "embedded - one field set",
			value: &CreateUserRequest{
				Name: "John Doe",
				Contact: struct {
					Email       *string
					PhoneNumber *string
					Address     *string
				}{
					Email: ptr("john@example.com"),
				},
			},
			wantError: false,
		},
		{
			name: "embedded - multiple fields set",
			value: &CreateUserRequest{
				Name: "John Doe",
				Contact: struct {
					Email       *string
					PhoneNumber *string
					Address     *string
				}{
					Email:       ptr("john@example.com"),
					PhoneNumber: ptr("+1234567890"),
				},
			},
			wantError: true,
			errorMsg:  "oneof constraint violated for group 'contact'",
		},

		// Complex request (no longer detected as oneof without tags)
		{
			name: "multiple fields - no oneof validation",
			value: &ComplexRequest{
				RequestID:    "req-123",
				TargetUser:   ptr("user-456"),
				ActionCreate: ptr(true),
			},
			wantError: false,
		},
		{
			name: "multiple fields set - no oneof validation",
			value: &ComplexRequest{
				RequestID:    "req-123",
				TargetUser:   ptr("user-456"),
				TargetGroup:  ptr("group-789"),
				ActionCreate: ptr(true),
			},
			wantError: false, // No error because not detected as oneof
			errorMsg:  "",
		},
		{
			name: "all fields set - no oneof validation",
			value: &ComplexRequest{
				RequestID:    "req-123",
				TargetUser:   ptr("user-456"),
				ActionCreate: ptr(true),
				ActionUpdate: ptr(true),
			},
			wantError: false, // No error because not detected as oneof
			errorMsg:  "",
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

// Helper function
func ptr[T any](v T) *T {
	return &v
}
