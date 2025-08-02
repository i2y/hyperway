package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

// Test Empty type
func TestEmptyType(t *testing.T) {
	// Test with struct{}
	type EmptyRequest struct {
		Empty struct{} `json:"empty"`
	}

	type EmptyResponse struct {
		Success bool `json:"success"`
	}

	// Create service
	svc := rpc.NewService("EmptyService", rpc.WithPackage("test.v1"))

	// Handler
	handler := func(ctx context.Context, req *EmptyRequest) (*EmptyResponse, error) {
		return &EmptyResponse{Success: true}, nil
	}

	// Register handler
	if err := rpc.Register(svc, "TestEmpty", handler); err != nil {
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
		"empty": map[string]any{}, // Empty object for struct{}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		server.URL+"/test.v1.EmptyService/TestEmpty",
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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result EmptyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if !result.Success {
		t.Error("Expected success=true")
	}
}

// Test proto:"empty" tag
func TestEmptyTag(t *testing.T) {
	type TaggedEmptyRequest struct {
		// String field tagged as empty
		ShouldBeEmpty string `json:"should_be_empty" proto:"empty"`
		// Normal string field
		NormalString string `json:"normal_string"`
	}

	type TaggedEmptyResponse struct {
		ReceivedNormal string `json:"received_normal"`
	}

	// Create service
	svc := rpc.NewService("TaggedEmptyService", rpc.WithPackage("test.v1"))

	// Handler
	handler := func(ctx context.Context, req *TaggedEmptyRequest) (*TaggedEmptyResponse, error) {
		return &TaggedEmptyResponse{
			ReceivedNormal: req.NormalString,
		}, nil
	}

	// Register handler
	if err := rpc.Register(svc, "TestTaggedEmpty", handler); err != nil {
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

	// Test request - for proto:"empty" tag, we send an empty value
	reqBody := map[string]any{
		"should_be_empty": nil, // null for proto:"empty" field
		"normal_string":   "test value",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		server.URL+"/test.v1.TaggedEmptyService/TestTaggedEmpty",
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

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body.String())
	}

	var result TaggedEmptyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if result.ReceivedNormal != "test value" {
		t.Errorf("Expected received_normal='test value', got %s", result.ReceivedNormal)
	}
}

// Test Any type
func TestAnyType(t *testing.T) {
	t.Skip("Any type JSON serialization requires special handling")
}
