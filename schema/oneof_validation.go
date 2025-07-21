package schema

import (
	"fmt"
	"reflect"
	"strings"
)

// OneofValidator validates oneof constraints
type OneofValidator struct {
	groups []OneofGroup
}

// NewOneofValidator creates a new oneof validator for a struct type
func NewOneofValidator(structType reflect.Type) *OneofValidator {
	return &OneofValidator{
		groups: detectOneofGroups(structType),
	}
}

// Validate checks that oneof constraints are satisfied
func (v *OneofValidator) Validate(value any) error {
	if len(v.groups) == 0 {
		return nil // No oneof groups to validate
	}

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %v", val.Kind())
	}

	// Validate each oneof group
	for _, group := range v.groups {
		if err := v.validateOneofGroup(val, &group); err != nil {
			return err
		}
	}

	return nil
}

// validateOneofGroup validates a single oneof group
func (v *OneofValidator) validateOneofGroup(structVal reflect.Value, group *OneofGroup) error {
	setCount := 0
	// Pre-allocate with estimated capacity
	setFields := make([]string, 0, len(group.Fields))

	// All oneofs are now tagged structs
	// Find the struct field
	// Note: strings.Title is deprecated but works for our simple use case
	//nolint:staticcheck // SA1019: strings.Title is deprecated but sufficient for field names
	titleName := strings.Title(group.Name)
	structField := structVal.FieldByName(titleName)
	if !structField.IsValid() {
		return nil // Field not found, skip
	}

	// If it's a pointer to struct, dereference
	if structField.Kind() == reflect.Ptr {
		if structField.IsNil() {
			return nil // Nil struct means no fields are set
		}
		structField = structField.Elem()
	}

	// Check fields within the struct
	for fieldName, fieldIdx := range group.Fields {
		field := structField.Field(fieldIdx)
		if !isFieldSet(field) {
			continue
		}
		setCount++
		setFields = append(setFields, fieldName)
	}

	// Validate oneof constraint
	if setCount > 1 {
		return fmt.Errorf("oneof constraint violated for group '%s': multiple fields are set (%s)",
			group.Name, strings.Join(setFields, ", "))
	}

	return nil
}

// isFieldSet checks if a field has a non-zero value
func isFieldSet(field reflect.Value) bool {
	switch field.Kind() { //nolint:exhaustive // Other types handled in default case
	case reflect.Ptr:
		return !field.IsNil()
	case reflect.Slice, reflect.Map:
		return !field.IsNil() && field.Len() > 0
	case reflect.String:
		return field.String() != ""
	case reflect.Interface:
		return !field.IsNil()
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return !field.IsZero()
	case reflect.Struct:
		return !field.IsZero()
	default:
		// For any other types (Invalid, Uintptr, Array, Chan, Func, UnsafePointer), check if it's the zero value
		return !field.IsZero()
	}
}

// ValidateOneof is a convenience function to validate oneof constraints
func ValidateOneof(structType reflect.Type, value any) error {
	validator := NewOneofValidator(structType)
	return validator.Validate(value)
}
