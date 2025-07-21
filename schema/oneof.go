package schema

import (
	"reflect"
	"strings"
)

// OneofGroup represents a detected oneof group
type OneofGroup struct {
	Name   string             // Group name (e.g., "identifier")
	Fields map[string]int     // Field name -> field index in struct
	Type   OneofDetectionType // How this oneof was detected
}

// OneofDetectionType indicates how a oneof group was detected
type OneofDetectionType int

const (
	// OneofTypeStructTag detected via struct tag
	OneofTypeStructTag OneofDetectionType = iota
)

// detectOneofGroups analyzes a struct type and returns all detected oneof groups
func detectOneofGroups(structType reflect.Type) []OneofGroup {
	// Only detect explicitly tagged oneofs with hyperway:"oneof"
	return detectTaggedOneofGroups(structType)
}

// detectTaggedOneofGroups looks for structs with explicit oneof tags
func detectTaggedOneofGroups(structType reflect.Type) []OneofGroup {
	var groups []OneofGroup

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Check for hyperway:"oneof" tag
		if tag := field.Tag.Get("hyperway"); tag == "oneof" {
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			if fieldType.Kind() != reflect.Struct {
				continue
			}

			fields := make(map[string]int)
			for j := 0; j < fieldType.NumField(); j++ {
				subField := fieldType.Field(j)
				if subField.IsExported() {
					fields[subField.Name] = j
				}
			}

			if len(fields) >= 2 {
				groups = append(groups, OneofGroup{
					Name:   strings.ToLower(field.Name),
					Fields: fields,
					Type:   OneofTypeStructTag,
				})
			}
		}
	}

	return groups
}
