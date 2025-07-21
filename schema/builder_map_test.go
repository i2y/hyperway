package schema_test

import (
	"reflect"
	"testing"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_MapFields(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type MapStruct struct {
		StringMap map[string]string  `json:"string_map"`
		IntMap    map[string]int32   `json:"int_map"`
		BoolMap   map[string]bool    `json:"bool_map"`
		FloatMap  map[string]float64 `json:"float_map"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(MapStruct{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 4 {
		t.Errorf("Expected 4 fields, got %d", fields.Len())
	}

	// Check that all fields are maps
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if !field.IsMap() {
			t.Errorf("Field %s should be a map", field.Name())
		}

		// Verify map entry structure
		mapEntry := field.Message()
		if mapEntry == nil {
			t.Errorf("Map field %s has no message descriptor", field.Name())
			continue
		}

		// Map entries should have exactly 2 fields: key and value
		if mapEntry.Fields().Len() != 2 {
			t.Errorf("Map entry for %s should have 2 fields, got %d",
				field.Name(), mapEntry.Fields().Len())
		}

		// Check key field
		keyField := mapEntry.Fields().ByName("key")
		if keyField == nil {
			t.Errorf("Map entry for %s missing 'key' field", field.Name())
		}

		// Check value field
		valueField := mapEntry.Fields().ByName("value")
		if valueField == nil {
			t.Errorf("Map entry for %s missing 'value' field", field.Name())
		}
	}
}

func TestBuilder_ComplexMapTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type ComplexMapStruct struct {
		// Map with various key types
		IntKeyMap    map[int32]string `json:"int_key_map"`
		Uint64KeyMap map[uint64]bool  `json:"uint64_key_map"`

		// Map with struct value (should fail currently)
		// StructMap map[string]NestedValue `json:"struct_map"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(ComplexMapStruct{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 2 {
		t.Errorf("Expected 2 fields, got %d", fields.Len())
	}
}

func TestBuilder_EmptyMap(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type EmptyMapStruct struct {
		EmptyMap   map[string]string `json:"empty_map"`
		OtherField string            `json:"other_field"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(EmptyMapStruct{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 2 {
		t.Errorf("Expected 2 fields, got %d", fields.Len())
	}

	// Check map field
	mapField := fields.ByName("empty_map")
	if mapField == nil {
		t.Fatal("empty_map field not found")
	}
	if !mapField.IsMap() {
		t.Error("empty_map should be a map")
	}
}
