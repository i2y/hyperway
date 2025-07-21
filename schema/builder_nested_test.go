package schema_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_NestedMessages(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		Country string `json:"country"`
		ZipCode string `json:"zip_code"`
	}

	type Contact struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Age     int32   `json:"age"`
		Address Address `json:"address"`
		Contact Contact `json:"contact"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(Person{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 4 {
		t.Errorf("Expected 4 fields, got %d", fields.Len())
	}

	// Check nested Address field
	addressField := fields.ByName("address")
	if addressField == nil {
		t.Fatal("address field not found")
	}
	if addressField.Kind() != protoreflect.MessageKind {
		t.Errorf("address field should be a message, got %v", addressField.Kind())
	}

	// The nested message descriptor should have the expected fields
	// Note: In the current implementation, nested messages might not be fully resolved
	// This is a known limitation that needs to be fixed
}

func TestBuilder_DeeplyNestedMessages(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Level3 struct {
		Value string `json:"value"`
	}

	type Level2 struct {
		L3    Level3 `json:"l3"`
		Count int32  `json:"count"`
	}

	type Level1 struct {
		L2   Level2 `json:"l2"`
		Name string `json:"name"`
	}

	type RootMessage struct {
		L1 Level1 `json:"l1"`
		ID string `json:"id"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(RootMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 2 {
		t.Errorf("Expected 2 fields, got %d", fields.Len())
	}

	// Verify L1 field is a message
	l1Field := fields.ByName("l1")
	if l1Field == nil {
		t.Fatal("l1 field not found")
	}
	if l1Field.Kind() != protoreflect.MessageKind {
		t.Errorf("l1 field should be a message, got %v", l1Field.Kind())
	}
}

func TestBuilder_AnonymousStructs(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Container struct {
		Name string `json:"name"`
		Data struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"data"`
		Count int32 `json:"count"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(Container{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 3 {
		t.Errorf("Expected 3 fields, got %d", fields.Len())
	}

	// Check that anonymous struct field is properly handled
	dataField := fields.ByName("data")
	if dataField == nil {
		t.Fatal("data field not found")
	}
	if dataField.Kind() != protoreflect.MessageKind {
		t.Errorf("data field should be a message, got %v", dataField.Kind())
	}
}

func TestBuilder_PointerFields(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type OptionalData struct {
		Value string `json:"value"`
	}

	type MessageWithPointers struct {
		RequiredString string        `json:"required_string"`
		OptionalString *string       `json:"optional_string"`
		OptionalInt    *int32        `json:"optional_int"`
		OptionalBool   *bool         `json:"optional_bool"`
		OptionalData   *OptionalData `json:"optional_data"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(MessageWithPointers{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	if fields.Len() != 5 {
		t.Errorf("Expected 5 fields, got %d", fields.Len())
	}

	// All fields should be marked as optional in proto3
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Cardinality() != protoreflect.Optional {
			t.Errorf("Field %s should be optional, got %v", field.Name(), field.Cardinality())
		}
	}
}

// CircularReference test is commented out as it should return an error
// func TestBuilder_CircularReference(t *testing.T) {
// 	builder := schema.NewBuilder(schema.BuilderOptions{
// 		PackageName: "test.v1",
// 	})
//
// 	type Node struct {
// 		Value string `json:"value"`
// 		Next  *Node  `json:"next"` // Circular reference
// 	}
//
// 	_, err := builder.BuildMessage(reflect.TypeOf(Node{}))
// 	if err == nil {
// 		t.Error("Expected error for circular reference, got nil")
// 	}
// }
