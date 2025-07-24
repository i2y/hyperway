package schema_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_ValidationTags(t *testing.T) {
	type ValidatedStruct struct {
		Name     string `json:"name" validate:"required,min=3,max=50"`
		Email    string `json:"email" validate:"required,email"`
		Age      int32  `json:"age" validate:"min=0,max=150"`
		Password string `json:"password" validate:"required,min=8"`
		URL      string `json:"url,omitempty" validate:"url"`
	}

	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	_, err := builder.BuildMessage(reflect.TypeOf(ValidatedStruct{}))
	if err != nil {
		t.Fatalf("Failed to build message with validation tags: %v", err)
	}

	// Get the file descriptor
	fdset := builder.GetFileDescriptorSet()

	// Find the message
	var msg *descriptorpb.DescriptorProto
	for _, file := range fdset.File {
		for _, m := range file.MessageType {
			if m.GetName() == "ValidatedStruct" {
				msg = m
				break
			}
		}
	}

	if msg == nil {
		t.Fatal("ValidatedStruct message not found")
	}

	// Check that all expected fields are present
	expectedFields := map[string]bool{
		"name":     false,
		"email":    false,
		"age":      false,
		"password": false,
		"url":      false,
	}

	for _, field := range msg.Field {
		fieldName := field.GetName()
		if _, ok := expectedFields[fieldName]; ok {
			expectedFields[fieldName] = true
		}
	}

	// Ensure all expected fields were found
	for name, found := range expectedFields {
		if !found {
			t.Errorf("Expected field %s not found", name)
		}
	}

	// Validation is now handled at runtime via struct tags,
	// not stored in the protobuf descriptor
}

func TestBuilder_ValidationWithComplexTypes(t *testing.T) {
	type Address struct {
		Street  string `json:"street" validate:"required"`
		City    string `json:"city" validate:"required"`
		ZipCode string `json:"zip_code" validate:"required,numeric,len=5"`
	}

	type User struct {
		ID       string            `json:"id" validate:"required,uuid"`
		Username string            `json:"username" validate:"required,alphanum,min=3,max=20"`
		Email    string            `json:"email" validate:"required,email"`
		Phone    string            `json:"phone" validate:"e164"`
		Address  Address           `json:"address" validate:"required"`
		Tags     []string          `json:"tags" validate:"min=1,max=10,dive,min=1,max=20"`
		Settings map[string]string `json:"settings" validate:"max=50"`
	}

	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	md, err := builder.BuildMessage(reflect.TypeOf(User{}))
	if err != nil {
		t.Fatalf("Failed to build message: %v", err)
	}

	if md == nil {
		t.Fatal("Expected message descriptor, got nil")
	}

	// Build the FileDescriptorSet
	fdset := builder.GetFileDescriptorSet()

	// Verify both User and Address messages exist
	messageNames := make(map[string]bool)
	for _, file := range fdset.File {
		for _, msg := range file.MessageType {
			messageNames[msg.GetName()] = true
			t.Logf("Found message: %s", msg.GetName())

			// Validation metadata is no longer stored in JsonName
			// The validation is handled at runtime via struct tags
		}
	}

	expectedMessages := []string{"User", "Address"}
	for _, name := range expectedMessages {
		if !messageNames[name] {
			t.Errorf("Expected message %s not found", name)
		}
	}
}
