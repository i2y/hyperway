package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test types
type TestRequest struct {
	Name string `json:"name"`
}

type TestResponse struct {
	Message string `json:"message"`
}

// Test handler
func testHandler(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	return &TestResponse{
		Message: "Hello, " + req.Name,
	}, nil
}

func TestJSONRPCHandler(t *testing.T) {
	// Create service with JSON-RPC enabled
	svc := NewService("TestService",
		WithPackage("test.v1"),
		WithJSONRPC("/jsonrpc"),
	)

	// Register test method
	MustRegister(svc, "SayHello", testHandler)

	// Create gateway
	gw, err := NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Test single JSON-RPC request
	t.Run("SingleRequest", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "SayHello",
			Params:  json.RawMessage(`{"name": "World"}`),
			ID:      1,
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gw.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var resp JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Got error response: %+v", resp.Error)
		}

		var result TestResponse
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if result.Message != "Hello, World" {
			t.Fatalf("Expected 'Hello, World', got '%s'", result.Message)
		}
	})

	// Test batch request
	t.Run("BatchRequest", func(t *testing.T) {
		reqs := []JSONRPCRequest{
			{
				JSONRPC: "2.0",
				Method:  "SayHello",
				Params:  json.RawMessage(`{"name": "Alice"}`),
				ID:      1,
			},
			{
				JSONRPC: "2.0",
				Method:  "SayHello",
				Params:  json.RawMessage(`{"name": "Bob"}`),
				ID:      2,
			},
		}

		body, _ := json.Marshal(reqs)
		httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gw.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var responses []JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&responses); err != nil {
			t.Fatalf("Failed to decode batch response: %v", err)
		}

		if len(responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(responses))
		}

		// Check both responses
		expectedMessages := map[interface{}]string{
			float64(1): "Hello, Alice", // JSON unmarshals numbers as float64
			float64(2): "Hello, Bob",
		}

		for _, resp := range responses {
			if resp.Error != nil {
				t.Fatalf("Got error response: %+v", resp.Error)
			}

			var result TestResponse
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			expected, ok := expectedMessages[resp.ID]
			if !ok {
				t.Fatalf("Unexpected response ID: %v", resp.ID)
			}

			if result.Message != expected {
				t.Fatalf("Expected '%s', got '%s'", expected, result.Message)
			}
		}
	})

	// Test notification (no response expected)
	t.Run("Notification", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "SayHello",
			Params:  json.RawMessage(`{"name": "Notification"}`),
			// No ID field - this makes it a notification
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gw.ServeHTTP(w, httpReq)

		if w.Code != http.StatusNoContent {
			t.Fatalf("Expected status 204 for notification, got %d", w.Code)
		}

		if w.Body.Len() > 0 {
			t.Fatalf("Expected empty body for notification, got: %s", w.Body.String())
		}
	})

	// Test method not found
	t.Run("MethodNotFound", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "NonExistentMethod",
			Params:  json.RawMessage(`{}`),
			ID:      1,
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gw.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var resp JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Error == nil {
			t.Fatal("Expected error response, got success")
		}

		if resp.Error.Code != JSONRPCMethodNotFound {
			t.Fatalf("Expected method not found error (%d), got %d", JSONRPCMethodNotFound, resp.Error.Code)
		}
	})
}
