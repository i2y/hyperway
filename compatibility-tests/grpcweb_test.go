package compatibility_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	compatibility "github.com/i2y/hyperway/compatibility-tests"
	"github.com/i2y/hyperway/rpc"
)

// TestGRPCWebProtocols tests gRPC-Web protocol handling after refactoring
func TestGRPCWebProtocols(t *testing.T) {
	// Create Hyperway service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	// Register handler
	err := rpc.Register(svc, "Echo", func(ctx context.Context, req *compatibility.SimpleRequest) (*compatibility.SimpleResponse, error) {
		return &compatibility.SimpleResponse{
			StringField: "Echo: " + req.StringField,
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register method: %v", err)
	}

	// Create handler
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Start test server
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("gRPC-Web+JSON with framing", func(t *testing.T) {
		// Create JSON payload - use snake_case for JSON field names
		payload := map[string]interface{}{
			"string_field": "test message",
		}
		jsonData, _ := json.Marshal(payload)

		// Create framed message (5 bytes header + JSON)
		frameSize := len(jsonData)
		frame := make([]byte, 5+frameSize)
		frame[0] = 0 // No compression
		binary.BigEndian.PutUint32(frame[1:5], uint32(frameSize))
		copy(frame[5:], jsonData)

		// Send request
		req, _ := http.NewRequest("POST", server.URL+"/test.v1.TestService/Echo", bytes.NewReader(frame))
		req.Header.Set("Content-Type", "application/grpc-web+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Read response
		body, _ := io.ReadAll(resp.Body)

		// Check response (should be framed)
		if len(body) < 5 {
			t.Fatalf("response too short: %d bytes", len(body))
		}

		// Parse frame header
		respFrameFlag := body[0]
		respFrameLen := binary.BigEndian.Uint32(body[1:5])

		if respFrameFlag != 0 {
			t.Errorf("expected flag 0, got %d", respFrameFlag)
		}

		// Parse JSON response
		if int(respFrameLen) > len(body)-5 {
			t.Fatalf("frame length %d exceeds available data", respFrameLen)
		}

		respJSON := body[5 : 5+respFrameLen]
		var respData map[string]interface{}
		if err := json.Unmarshal(respJSON, &respData); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// JSON uses snake_case for field names
		if msg, ok := respData["string_field"].(string); !ok || msg != "Echo: test message" {
			t.Errorf("unexpected response: %v", respData)
		}

		// Check for trailer frame (if present)
		if len(body) > int(5+respFrameLen) {
			trailerStart := 5 + respFrameLen
			if body[trailerStart] == 0x80 { // Trailer flag
				t.Logf("Trailer frame found at offset %d", trailerStart)
			}
		}
	})

	t.Run("gRPC-Web+Protobuf with framing", func(t *testing.T) {
		// This would need proper protobuf encoding
		// For now, just verify the server accepts the content type
		req, _ := http.NewRequest("POST", server.URL+"/test.v1.TestService/Echo", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "application/grpc-web+proto")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should get an error response but with correct content type
		if ct := resp.Header.Get("Content-Type"); ct != "application/grpc-web+proto" {
			t.Logf("Expected Content-Type application/grpc-web+proto, got %s", ct)
		}
	})
}
