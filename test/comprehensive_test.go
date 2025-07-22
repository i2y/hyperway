// Package test provides comprehensive integration tests for hyperway
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/i2y/hyperway/rpc"
)

// Test data types to verify type support
type TestRequest struct {
	// Primitive types
	StringField string  `json:"string_field" validate:"required,min=3"`
	IntField    int32   `json:"int_field" validate:"min=0,max=100"`
	BoolField   bool    `json:"bool_field"`
	FloatField  float64 `json:"float_field"`

	// Complex types
	TimeField     time.Time         `json:"time_field"`
	DurationField time.Duration     `json:"duration_field"`
	BytesField    []byte            `json:"bytes_field"`
	ArrayField    []string          `json:"array_field"`
	MapField      map[string]string `json:"map_field"`

	// Nested types
	NestedField *NestedStruct `json:"nested_field,omitempty"`

	// Optional field
	OptionalField *string `json:"optional_field,omitempty"`

	// Well-known Empty type
	EmptyField struct{} `json:"empty_field"`
}

type NestedStruct struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Echo    string `json:"echo"`
}

// Test handlers
func echoHandler(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	return &TestResponse{
		Success: true,
		Message: fmt.Sprintf("Received: %s", req.StringField),
		Echo:    req.StringField,
	}, nil
}

func errorHandler(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	if req.StringField == "error" {
		return nil, fmt.Errorf("test error")
	}
	return &TestResponse{Success: true}, nil
}

// TestBasicFunctionality tests core RPC functionality
func TestBasicFunctionality(t *testing.T) {
	// Create service
	svc := rpc.NewService("TestService",
		rpc.WithPackage("test.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Register methods
	if err := rpc.Register(svc, "Echo", echoHandler); err != nil {
		t.Fatal(err)
	}
	if err := rpc.Register(svc, "Error", errorHandler); err != nil {
		t.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Start test server
	server := httptest.NewServer(h2c.NewHandler(gateway, &http2.Server{}))
	defer server.Close()

	t.Run("JSON Request", func(t *testing.T) {
		reqBody := TestRequest{
			StringField:   "hello world",
			IntField:      42,
			BoolField:     true,
			FloatField:    3.14,
			TimeField:     time.Now(),
			DurationField: 5 * time.Second,
			BytesField:    []byte("test"),
			ArrayField:    []string{"a", "b", "c"},
			MapField:      map[string]string{"key": "value"},
			NestedField: &NestedStruct{
				ID:   "123",
				Name: "nested",
			},
			EmptyField: struct{}{},
		}

		jsonData, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			server.URL+"/test.v1.TestService/Echo",
			bytes.NewReader(jsonData),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result TestResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !result.Success {
			t.Error("Expected success=true")
		}
		if result.Echo != "hello world" {
			t.Errorf("Expected echo='hello world', got '%s'", result.Echo)
		}
	})

	t.Run("Validation", func(t *testing.T) {
		// Test with invalid data (string too short)
		reqBody := TestRequest{
			StringField: "hi", // min=3
			IntField:    150,  // max=100
		}

		jsonData, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			server.URL+"/test.v1.TestService/Echo",
			bytes.NewReader(jsonData),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Should fail validation
		if resp.StatusCode == http.StatusOK {
			t.Error("Expected validation error")
		}
	})

	t.Run("Error Handling", func(t *testing.T) {
		reqBody := TestRequest{
			StringField: "error",
		}

		jsonData, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			server.URL+"/test.v1.TestService/Error",
			bytes.NewReader(jsonData),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("test error")) {
			t.Errorf("Expected error message, got: %s", string(body))
		}
	})
}

// TestProtocolSupport tests multi-protocol support
func TestProtocolSupport(t *testing.T) {
	svc := rpc.NewService("ProtocolTest", rpc.WithPackage("protocol.v1"))
	if err := rpc.Register(svc, "Test", echoHandler); err != nil {
		t.Fatal(err)
	}

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(h2c.NewHandler(gateway, &http2.Server{}))
	defer server.Close()

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "Connect RPC JSON",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name: "Connect Protocol",
			headers: map[string]string{
				"Content-Type":             "application/json",
				"Connect-Protocol-Version": "1",
			},
		},
		// Note: Testing actual gRPC requires grpcurl or a gRPC client
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := `{"string_field":"test protocol"}`
			req, err := http.NewRequestWithContext(
				context.Background(),
				"POST",
				server.URL+"/protocol.v1.ProtocolTest/Test",
				bytes.NewReader([]byte(reqBody)),
			)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
			}
		})
	}
}

// TestDataTypeSupport tests various data type support
func TestDataTypeSupport(t *testing.T) {
	type DataTypeTest struct {
		// All basic types
		String  string    `json:"string"`
		Bool    bool      `json:"bool"`
		Int32   int32     `json:"int32"`
		Int64   int64     `json:"int64"`
		Uint32  uint32    `json:"uint32"`
		Uint64  uint64    `json:"uint64"`
		Float32 float32   `json:"float32"`
		Float64 float64   `json:"float64"`
		Bytes   []byte    `json:"bytes"`
		Time    time.Time `json:"time"`

		// Collections
		StringSlice []string          `json:"string_slice"`
		IntSlice    []int32           `json:"int_slice"`
		StringMap   map[string]string `json:"string_map"`
		IntMap      map[string]int32  `json:"int_map"`

		// Nested
		Nested     *NestedStruct           `json:"nested"`
		NestedList []NestedStruct          `json:"nested_list"` // Changed from []*NestedStruct
		NestedMap  map[string]NestedStruct `json:"nested_map"`  // Changed from map[string]*NestedStruct

		// Optional fields
		OptString *string       `json:"opt_string,omitempty"`
		OptInt    *int32        `json:"opt_int,omitempty"`
		OptBool   *bool         `json:"opt_bool,omitempty"`
		OptNested *NestedStruct `json:"opt_nested,omitempty"`
	}

	handler := func(ctx context.Context, req *DataTypeTest) (*DataTypeTest, error) {
		// Echo back the request
		return req, nil
	}

	svc := rpc.NewService("DataTypeService", rpc.WithPackage("datatype.v1"))
	if err := rpc.Register(svc, "Echo", handler); err != nil {
		t.Fatal(err)
	}

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test with all fields populated
	testStr := "optional"
	testInt := int32(42)
	testBool := true

	req := DataTypeTest{
		String:  "hello",
		Bool:    true,
		Int32:   32,
		Int64:   64,
		Uint32:  32,
		Uint64:  64,
		Float32: 3.14,
		Float64: 2.718,
		Bytes:   []byte("test bytes"),
		Time:    time.Now(),

		StringSlice: []string{"a", "b", "c"},
		IntSlice:    []int32{1, 2, 3},
		StringMap:   map[string]string{"k1": "v1", "k2": "v2"},
		IntMap:      map[string]int32{"a": 1, "b": 2},

		Nested: &NestedStruct{ID: "1", Name: "nested"},
		NestedList: []NestedStruct{
			{ID: "1", Name: "first"},
			{ID: "2", Name: "second"},
		},
		NestedMap: map[string]NestedStruct{
			"item1": {ID: "1", Name: "item1"},
			"item2": {ID: "2", Name: "item2"},
		},

		OptString: &testStr,
		OptInt:    &testInt,
		OptBool:   &testBool,
		OptNested: &NestedStruct{ID: "opt", Name: "optional"},
	}

	jsonData, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		server.URL+"/datatype.v1.DataTypeService/Echo",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result DataTypeTest
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all fields were preserved
	if result.String != req.String {
		t.Errorf("String field mismatch")
	}
	if len(result.StringSlice) != len(req.StringSlice) {
		t.Errorf("StringSlice length mismatch")
	}
	if len(result.StringMap) != len(req.StringMap) {
		t.Errorf("StringMap length mismatch")
	}
	if result.Nested == nil || result.Nested.ID != req.Nested.ID {
		t.Errorf("Nested field mismatch")
	}
	if result.OptString == nil || *result.OptString != *req.OptString {
		t.Errorf("Optional string field mismatch")
	}
}

// TestOpenAPIGeneration tests OpenAPI spec generation
func TestOpenAPIGeneration(t *testing.T) {
	svc := rpc.NewService("OpenAPITest", rpc.WithPackage("openapi.v1"))
	if err := rpc.Register(svc, "Test", echoHandler); err != nil {
		t.Fatal(err)
	}

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Request OpenAPI spec
	req, err := http.NewRequestWithContext(
		context.Background(),
		"GET",
		server.URL+"/openapi.json",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get OpenAPI spec: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for OpenAPI spec, got %d", resp.StatusCode)
	}

	var openapi map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&openapi); err != nil {
		t.Fatalf("Failed to decode OpenAPI spec: %v", err)
	}

	// Verify OpenAPI structure
	if openapi["openapi"] == nil {
		t.Error("Missing openapi version field")
	}
	if openapi["paths"] == nil {
		t.Error("Missing paths field")
	}
	if openapi["components"] == nil {
		t.Error("Missing components field")
	}
}
