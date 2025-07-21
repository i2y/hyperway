package codec_test

import (
	"testing"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/codec"
)

// createTestDescriptor creates a simple test message descriptor.
func createTestDescriptor() (protoreflect.MessageDescriptor, error) {
	msgProto := &descriptorpb.DescriptorProto{
		Name: proto.String("TestMessage"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:   proto.String("id"),
				Number: proto.Int32(1),
				Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
				Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
			},
			{
				Name:   proto.String("value"),
				Number: proto.Int32(2),
				Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_INT64),
				Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
			},
			{
				Name:   proto.String("active"),
				Number: proto.Int32(3),
				Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_BOOL),
				Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
			},
		},
	}

	fileProto := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("test.proto"),
		Package:     proto.String("test.v1"),
		MessageType: []*descriptorpb.DescriptorProto{msgProto},
		Syntax:      proto.String("proto3"),
	}

	file, err := protodesc.NewFile(fileProto, nil)
	if err != nil {
		return nil, err
	}

	return file.Messages().ByName("TestMessage"), nil
}

func TestCodec_MarshalUnmarshal(t *testing.T) {
	md, err := createTestDescriptor()
	if err != nil {
		t.Fatalf("Failed to create test descriptor: %v", err)
	}

	c, err := codec.New(md, codec.DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create codec: %v", err)
	}

	// Since hyperpb messages are read-only, we'll create test data manually
	// Create a message with protobuf encoding
	testData := []byte{
		0x0a, 0x08, // field 1 (id), length 8
		't', 'e', 's', 't', '-', '1', '2', '3',
		0x10, 0x2a, // field 2 (value), varint 42
		0x18, 0x01, // field 3 (active), varint 1 (true)
	}

	// Unmarshal
	decoded, err := c.Unmarshal(testData)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Type assert to *hyperpb.Message to use Range method
	hyperpbMsg, ok := decoded.(*hyperpb.Message)
	if !ok {
		t.Fatal("Expected decoded message to be *hyperpb.Message")
	}

	// Verify fields using Range method - hyperpb.Message.Get has issues
	expectedFields := map[string]any{
		"id":     "test-123",
		"value":  int64(42),
		"active": true,
	}

	foundFields := make(map[string]bool)
	hyperpbMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := string(fd.Name())
		foundFields[name] = true

		switch name {
		case "id":
			if v.String() != expectedFields["id"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["id"], v.String())
			}
		case "value":
			if v.Int() != expectedFields["value"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["value"], v.Int())
			}
		case "active":
			if v.Bool() != expectedFields["active"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["active"], v.Bool())
			}
		}
		return true
	})

	// Check all expected fields were found
	for field := range expectedFields {
		if !foundFields[field] {
			t.Errorf("Expected field '%s' not found in message", field)
		}
	}
}

func TestCodec_JSON(t *testing.T) {
	md, err := createTestDescriptor()
	if err != nil {
		t.Fatalf("Failed to create test descriptor: %v", err)
	}

	c, err := codec.New(md, codec.DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create codec: %v", err)
	}

	// Test JSON unmarshaling - note: proto3 doesn't encode default values (false, 0, "")
	jsonData := []byte(`{"id":"json-test","value":100,"active":true}`)

	// Unmarshal from JSON
	decoded, err := c.UnmarshalFromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Type assert to *hyperpb.Message
	hyperpbMsg, ok := decoded.(*hyperpb.Message)
	if !ok {
		t.Fatal("Expected decoded message to be *hyperpb.Message")
	}

	// Debug: List all fields found
	t.Log("Fields found in JSON decoded message:")
	hyperpbMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		t.Logf("  - %s: %v", fd.Name(), v)
		return true
	})

	// Verify fields using Range method
	expectedFields := map[string]any{
		"id":     "json-test",
		"value":  int64(100),
		"active": true,
	}

	foundFields := make(map[string]bool)
	hyperpbMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := string(fd.Name())
		foundFields[name] = true

		switch name {
		case "id":
			if v.String() != expectedFields["id"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["id"], v.String())
			}
		case "value":
			if v.Int() != expectedFields["value"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["value"], v.Int())
			}
		case "active":
			if v.Bool() != expectedFields["active"] {
				t.Errorf("Field %s: expected %v, got %v", name, expectedFields["active"], v.Bool())
			}
		}
		return true
	})

	// Check all expected fields were found
	for field := range expectedFields {
		if !foundFields[field] {
			t.Errorf("Expected field '%s' not found in message", field)
		}
	}
}

func TestCodec_Pooling(t *testing.T) {
	md, err := createTestDescriptor()
	if err != nil {
		t.Fatalf("Failed to create test descriptor: %v", err)
	}

	opts := codec.DefaultOptions()
	opts.EnablePooling = true
	opts.PoolSize = 5

	c, err := codec.New(md, opts)
	if err != nil {
		t.Fatalf("Failed to create codec: %v", err)
	}

	// Get and release messages
	messages := make([]proto.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = c.NewMessage()
	}

	// Release all messages
	for _, msg := range messages {
		c.ReleaseMessage(msg)
	}

	// Get messages again - should reuse from pool
	for i := 0; i < 5; i++ {
		msg := c.NewMessage()
		c.ReleaseMessage(msg)
	}
}

func BenchmarkCodec_Marshal(b *testing.B) {
	// Benchmarking unmarshal since hyperpb messages are read-only
	b.Skip("Skipping marshal benchmark - hyperpb messages are read-only")
}

func BenchmarkCodec_Unmarshal(b *testing.B) {
	md, err := createTestDescriptor()
	if err != nil {
		b.Fatalf("Failed to create test descriptor: %v", err)
	}

	c, err := codec.New(md, codec.DefaultOptions())
	if err != nil {
		b.Fatalf("Failed to create codec: %v", err)
	}

	// Create test data
	testData := []byte{
		0x0a, 0x0a, // field 1 (id), length 10
		'b', 'e', 'n', 'c', 'h', '-', 't', 'e', 's', 't',
		0x10, 0xc0, 0xc4, 0x07, // field 2 (value), varint 123456
		0x18, 0x01, // field 3 (active), varint 1 (true)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := c.Unmarshal(testData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions.
func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}

func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}
