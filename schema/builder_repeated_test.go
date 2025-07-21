package schema_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_RepeatedFields(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type RepeatedFieldsStruct struct {
		StringList []string  `json:"string_list"`
		IntList    []int32   `json:"int_list"`
		FloatList  []float64 `json:"float_list"`
		BoolList   []bool    `json:"bool_list"`
		BytesList  [][]byte  `json:"bytes_list"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(RepeatedFieldsStruct{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 5 {
		t.Errorf("Expected 5 fields, got %d", fields.Len())
	}

	// Check that all fields are repeated
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Cardinality() != protoreflect.Repeated {
			t.Errorf("Field %s should be repeated, got %v", field.Name(), field.Cardinality())
		}
		if field.IsList() != true {
			t.Errorf("Field %s should be a list", field.Name())
		}
	}

	// Verify specific field types
	stringListField := fields.ByName("string_list")
	if stringListField != nil && stringListField.Kind() != protoreflect.StringKind {
		t.Errorf("string_list should be string kind, got %v", stringListField.Kind())
	}

	intListField := fields.ByName("int_list")
	if intListField != nil && intListField.Kind() != protoreflect.Int32Kind {
		t.Errorf("int_list should be int32 kind, got %v", intListField.Kind())
	}
}

func TestBuilder_RepeatedMessages(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Item struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	type Order struct {
		OrderID string  `json:"order_id"`
		Items   []Item  `json:"items"`
		Total   float64 `json:"total"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(Order{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 3 {
		t.Errorf("Expected 3 fields, got %d", fields.Len())
	}

	// Check Items field
	itemsField := fields.ByName("items")
	if itemsField == nil {
		t.Fatal("items field not found")
	}
	if itemsField.Cardinality() != protoreflect.Repeated {
		t.Errorf("items field should be repeated, got %v", itemsField.Cardinality())
	}
	if itemsField.Kind() != protoreflect.MessageKind {
		t.Errorf("items field should be message kind, got %v", itemsField.Kind())
	}
}

func TestBuilder_BytesVsByteSlice(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type BytesTest struct {
		SingleBytes []byte   `json:"single_bytes"` // Should be bytes type
		BytesList   [][]byte `json:"bytes_list"`   // Should be repeated bytes
		UintList    []uint8  `json:"uint_list"`    // Should be treated as bytes
	}

	md, err := builder.BuildMessage(reflect.TypeOf(BytesTest{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 3 {
		t.Errorf("Expected 3 fields, got %d", fields.Len())
	}

	// Check SingleBytes field - should be bytes, not repeated
	singleBytesField := fields.ByName("single_bytes")
	if singleBytesField == nil {
		t.Fatal("single_bytes field not found")
	}
	if singleBytesField.Kind() != protoreflect.BytesKind {
		t.Errorf("single_bytes should be bytes kind, got %v", singleBytesField.Kind())
	}
	if singleBytesField.Cardinality() != protoreflect.Optional {
		t.Errorf("single_bytes should be optional, got %v", singleBytesField.Cardinality())
	}

	// Check BytesList field - should be repeated bytes
	bytesListField := fields.ByName("bytes_list")
	if bytesListField == nil {
		t.Fatal("bytes_list field not found")
	}
	if bytesListField.Kind() != protoreflect.BytesKind {
		t.Errorf("bytes_list should be bytes kind, got %v", bytesListField.Kind())
	}
	if bytesListField.Cardinality() != protoreflect.Repeated {
		t.Errorf("bytes_list should be repeated, got %v", bytesListField.Cardinality())
	}
}

func TestBuilder_EmptySlices(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type EmptySlicesStruct struct {
		EmptyStrings []string `json:"empty_strings"`
		EmptyInts    []int32  `json:"empty_ints"`
		Name         string   `json:"name"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(EmptySlicesStruct{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 3 {
		t.Errorf("Expected 3 fields, got %d", fields.Len())
	}

	// Empty slices should still be properly typed as repeated fields
	emptyStringsField := fields.ByName("empty_strings")
	if emptyStringsField == nil {
		t.Fatal("empty_strings field not found")
	}
	if emptyStringsField.Cardinality() != protoreflect.Repeated {
		t.Errorf("empty_strings should be repeated, got %v", emptyStringsField.Cardinality())
	}
}

func TestBuilder_MixedRepeatedTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Tag struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	type ComplexRepeated struct {
		Tags     []Tag             `json:"tags"`     // Repeated messages
		Metadata map[string]string `json:"metadata"` // Map (also repeated internally)
		Keywords []string          `json:"keywords"` // Repeated strings
		Scores   []float32         `json:"scores"`   // Repeated floats
	}

	md, err := builder.BuildMessage(reflect.TypeOf(ComplexRepeated{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 4 {
		t.Errorf("Expected 4 fields, got %d", fields.Len())
	}

	// Check Tags field
	tagsField := fields.ByName("tags")
	if tagsField == nil {
		t.Fatal("tags field not found")
	}
	if tagsField.Cardinality() != protoreflect.Repeated {
		t.Errorf("tags should be repeated, got %v", tagsField.Cardinality())
	}
	if tagsField.Kind() != protoreflect.MessageKind {
		t.Errorf("tags should be message kind, got %v", tagsField.Kind())
	}

	// Check Metadata field (map)
	metadataField := fields.ByName("metadata")
	if metadataField == nil {
		t.Fatal("metadata field not found")
	}
	if !metadataField.IsMap() {
		t.Error("metadata should be a map")
	}
}
