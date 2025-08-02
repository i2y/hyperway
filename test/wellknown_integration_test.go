package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/i2y/hyperway/rpc"
)

// Test types for Well-Known Types integration
type WKTRequest struct {
	// Struct field
	Config *structpb.Struct `json:"config"`
	// Value field
	Setting *structpb.Value `json:"setting"`
	// ListValue field
	Items *structpb.ListValue `json:"items"`
	// FieldMask field
	UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"`
}

type WKTResponse struct {
	Success bool             `json:"success"`
	Echo    *structpb.Struct `json:"echo"`
}

func TestWellKnownTypesIntegration(t *testing.T) {
	// Create service
	svc := rpc.NewService("WKTService", rpc.WithPackage("test.v1"))

	// Handler that echoes back the config
	handler := func(ctx context.Context, req *WKTRequest) (*WKTResponse, error) {
		return &WKTResponse{
			Success: true,
			Echo:    req.Config,
		}, nil
	}

	// Register handler
	if err := rpc.Register(svc, "Process", handler); err != nil {
		t.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatal(err)
	}

	// Create test server
	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test cases
	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "struct with nested values",
			body: map[string]any{
				"config": map[string]any{
					"theme": "dark",
					"nested": map[string]any{
						"enabled": true,
						"count":   42,
					},
				},
				"setting": "test", // structpb.Value is automatically wrapped
				"items": []any{ // structpb.ListValue is automatically wrapped
					"a",
					123,
					true,
				},
				"update_mask": map[string]any{
					"paths": []string{"theme", "nested.enabled"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal request
			reqBody, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatal(err)
			}

			// Make request
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
				server.URL+"/test.v1.WKTService/Process",
				bytes.NewReader(reqBody))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Check status
			if resp.StatusCode != http.StatusOK {
				body := new(bytes.Buffer)
				_, _ = body.ReadFrom(resp.Body)
				t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body.String())
			}

			// Parse response
			var result WKTResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}

			// Verify response
			if !result.Success {
				t.Error("Expected success=true")
			}
			if result.Echo == nil {
				t.Fatal("Expected non-nil echo")
			}

			// Check echo content
			if theme, ok := result.Echo.Fields["theme"]; !ok || theme.GetStringValue() != "dark" {
				t.Error("Expected theme=dark in echo")
			}
		})
	}
}

func TestWellKnownTypesWithMaps(t *testing.T) {
	// Test type with map of Values
	type MapRequest struct {
		Properties map[string]*structpb.Value `json:"properties"`
	}

	type MapResponse struct {
		Count int `json:"count"`
	}

	// Create service
	svc := rpc.NewService("MapService", rpc.WithPackage("test.v1"))

	// Handler that counts properties
	handler := func(ctx context.Context, req *MapRequest) (*MapResponse, error) {
		return &MapResponse{
			Count: len(req.Properties),
		}, nil
	}

	// Register handler
	if err := rpc.Register(svc, "CountProperties", handler); err != nil {
		t.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatal(err)
	}

	// Create test server
	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test request
	reqBody := map[string]any{
		"properties": map[string]any{
			"name":   map[string]any{"string_value": "test"},
			"age":    map[string]any{"number_value": 30},
			"active": map[string]any{"bool_value": true},
		},
	}

	// Marshal request
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	// Make request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		server.URL+"/test.v1.MapService/CountProperties",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result MapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if result.Count != 3 {
		t.Errorf("Expected count=3, got %d", result.Count)
	}
}

func TestFieldMaskProcessing(t *testing.T) {
	// Test type for field mask operations
	type UpdateRequest struct {
		Data       *structpb.Struct       `json:"data"`
		UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"`
	}

	type UpdateResponse struct {
		UpdatedPaths []string `json:"updated_paths"`
	}

	// Create service
	svc := rpc.NewService("UpdateService", rpc.WithPackage("test.v1"))

	// Handler that returns the paths from field mask
	handler := func(ctx context.Context, req *UpdateRequest) (*UpdateResponse, error) {
		return &UpdateResponse{
			UpdatedPaths: req.UpdateMask.GetPaths(),
		}, nil
	}

	// Register handler
	if err := rpc.Register(svc, "Update", handler); err != nil {
		t.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatal(err)
	}

	// Create test server
	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test request
	reqBody := map[string]any{
		"data": map[string]any{
			"field1": "value1",
			"field2": "value2",
		},
		"update_mask": map[string]any{
			"paths": []string{"field1", "nested.field3"},
		},
	}

	// Marshal request
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	// Make request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		server.URL+"/test.v1.UpdateService/Update",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	// Verify paths
	if len(result.UpdatedPaths) != 2 {
		t.Fatalf("Expected 2 paths, got %d", len(result.UpdatedPaths))
	}
	if result.UpdatedPaths[0] != "field1" || result.UpdatedPaths[1] != "nested.field3" {
		t.Errorf("Unexpected paths: %v", result.UpdatedPaths)
	}
}
