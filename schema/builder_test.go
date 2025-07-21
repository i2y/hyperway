package schema_test

import (
	"reflect"
	"testing"

	"github.com/i2y/hyperway/schema"
)

type TestStruct struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Age         int     `json:"age"`
	Active      bool    `json:"active"`
	Score       float64 `json:"score"`
	BinaryData  []byte  `json:"binary_data"`
	InternalVal string  `json:"-"`
}

type NestedStruct struct {
	User    TestStruct `json:"user"`
	Created int64      `json:"created"`
}

func TestBuilder_BuildMessage(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	tests := []struct {
		name    string
		typ     reflect.Type
		wantErr bool
	}{
		{
			name:    "basic struct",
			typ:     reflect.TypeOf(TestStruct{}),
			wantErr: false,
		},
		{
			name:    "pointer to struct",
			typ:     reflect.TypeOf(&TestStruct{}),
			wantErr: false,
		},
		{
			name:    "nested struct",
			typ:     reflect.TypeOf(NestedStruct{}),
			wantErr: false,
		},
		{
			name:    "non-struct type",
			typ:     reflect.TypeOf("string"),
			wantErr: true,
		},
		{
			name:    "slice type",
			typ:     reflect.TypeOf([]string{}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, err := builder.BuildMessage(tt.typ)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildMessage() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && md == nil {
				t.Errorf("BuildMessage() returned nil descriptor")
			}
		})
	}
}

func TestBuilder_Caching(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName:  "test.v1",
		MaxCacheSize: 10,
	})

	typ := reflect.TypeOf(TestStruct{})

	// First call should build
	md1, err := builder.BuildMessage(typ)
	if err != nil {
		t.Fatalf("First BuildMessage() failed: %v", err)
	}

	// Second call should return cached
	md2, err := builder.BuildMessage(typ)
	if err != nil {
		t.Fatalf("Second BuildMessage() failed: %v", err)
	}

	// Should be the same instance
	if md1 != md2 {
		t.Errorf("Expected cached descriptor, got different instance")
	}
}

func TestBuilder_FieldTypes(t *testing.T) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "test.v1",
	})

	type AllTypes struct {
		String  string  `json:"string"`
		Bool    bool    `json:"bool"`
		Int32   int32   `json:"int32"`
		Int64   int64   `json:"int64"`
		Uint32  uint32  `json:"uint32"`
		Uint64  uint64  `json:"uint64"`
		Float32 float32 `json:"float32"`
		Float64 float64 `json:"float64"`
		Bytes   []byte  `json:"bytes"`
	}

	md, err := builder.BuildMessage(reflect.TypeOf(AllTypes{}))
	if err != nil {
		t.Fatalf("BuildMessage() failed: %v", err)
	}

	fields := md.Fields()
	expectedFieldCount := 9

	if fields.Len() != expectedFieldCount {
		t.Errorf("Expected %d fields, got %d", expectedFieldCount, fields.Len())
	}
}

func BenchmarkBuilder_BuildMessage(b *testing.B) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName: "bench.v1",
	})

	typ := reflect.TypeOf(TestStruct{})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := builder.BuildMessage(typ)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuilder_BuildMessage_Cached(b *testing.B) {
	builder := schema.NewBuilder(schema.BuilderOptions{
		PackageName:  "bench.v1",
		MaxCacheSize: 100,
	})

	typ := reflect.TypeOf(TestStruct{})

	// Warm up cache
	_, _ = builder.BuildMessage(typ)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := builder.BuildMessage(typ)
		if err != nil {
			b.Fatal(err)
		}
	}
}
