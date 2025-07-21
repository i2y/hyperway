package schema_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/schema"
)

func TestBuilder_WellKnownTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type WellKnownMessage struct {
		// Time fields
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at"`
		DeletedAt *time.Time `json:"deleted_at,omitempty"`

		// Duration fields
		Timeout       time.Duration  `json:"timeout"`
		RetryInterval *time.Duration `json:"retry_interval"`

		// Empty type
		EmptyField struct{} `json:"empty_field"`

		// Empty type with tag
		TaggedEmpty string `json:"tagged_empty" proto:"empty"`

		// Regular fields for comparison
		ID     string `json:"id"`
		Count  int32  `json:"count"`
		Active bool   `json:"active"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(WellKnownMessage{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Get the FileDescriptorSet
	fdset := builder.GetFileDescriptorSet()
	if len(fdset.File) == 0 {
		t.Fatal("No files in FileDescriptorSet")
	}

	// Check imports
	var file *descriptorpb.FileDescriptorProto
	for _, f := range fdset.File {
		if strings.Contains(f.GetName(), "wellknownmessage") {
			file = f
			break
		}
	}

	if file == nil {
		t.Fatal("Could not find WellKnownMessage file")
	}

	// Verify imports
	expectedImports := map[string]bool{
		"google/protobuf/timestamp.proto": false,
		"google/protobuf/duration.proto":  false,
		"google/protobuf/empty.proto":     false,
	}

	for _, dep := range file.Dependency {
		if _, ok := expectedImports[dep]; ok {
			expectedImports[dep] = true
		}
	}

	for imp, found := range expectedImports {
		if !found {
			t.Errorf("Expected import %s not found", imp)
		}
	}

	// Verify field types
	fields := md.Fields()
	fieldTests := []struct {
		name         string
		expectedType string
	}{
		{"created_at", ".google.protobuf.Timestamp"},
		{"updated_at", ".google.protobuf.Timestamp"},
		{"deleted_at", ".google.protobuf.Timestamp"},
		{"timeout", ".google.protobuf.Duration"},
		{"retry_interval", ".google.protobuf.Duration"},
		{"empty_field", ".google.protobuf.Empty"},
		{"tagged_empty", ".google.protobuf.Empty"},
		{"id", ""},     // String type has no type name
		{"count", ""},  // Int32 type has no type name
		{"active", ""}, // Bool type has no type name
	}

	for _, tt := range fieldTests {
		t.Run(tt.name, func(t *testing.T) {
			field := fields.ByName(protoreflect.Name(tt.name))
			if field == nil {
				t.Fatalf("Field %s not found", tt.name)
			}

			if field.Kind() == protoreflect.MessageKind {
				typeName := string(field.Message().FullName())
				if typeName != strings.TrimPrefix(tt.expectedType, ".") {
					t.Errorf("Field %s: expected type %s, got %s", tt.name, tt.expectedType, typeName)
				}
			} else if tt.expectedType != "" {
				t.Errorf("Field %s: expected message type but got %v", tt.name, field.Kind())
			}
		})
	}
}

func TestBuilder_NestedWellKnownTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type Event struct {
		Timestamp time.Time         `json:"timestamp"`
		Duration  time.Duration     `json:"duration"`
		Metadata  map[string]string `json:"metadata"`
	}

	type Container struct {
		Events        []Event       `json:"events"`
		LastEventTime *time.Time    `json:"last_event_time"`
		TotalDuration time.Duration `json:"total_duration"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(Container{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	// Check that nested message also uses well-known types
	fields := md.Fields()

	// Check events field
	eventsField := fields.ByName("events")
	if eventsField == nil {
		t.Fatal("events field not found")
	}

	if eventsField.Cardinality() != protoreflect.Repeated {
		t.Errorf("events field should be repeated")
	}

	// Check that Event message was created with well-known types
	eventMsg := eventsField.Message()
	if eventMsg == nil {
		t.Fatal("Event message not found")
	}

	// Check Event fields
	timestampField := eventMsg.Fields().ByName("timestamp")
	if timestampField == nil {
		t.Fatal("timestamp field not found in Event")
	}
	if timestampField.Kind() != protoreflect.MessageKind {
		t.Errorf("timestamp should be a message type")
	}
	if !strings.Contains(string(timestampField.Message().FullName()), "google.protobuf.Timestamp") {
		t.Errorf("timestamp should be google.protobuf.Timestamp")
	}

	durationField := eventMsg.Fields().ByName("duration")
	if durationField == nil {
		t.Fatal("duration field not found in Event")
	}
	if durationField.Kind() != protoreflect.MessageKind {
		t.Errorf("duration should be a message type")
	}
	if !strings.Contains(string(durationField.Message().FullName()), "google.protobuf.Duration") {
		t.Errorf("duration should be google.protobuf.Duration")
	}
}
