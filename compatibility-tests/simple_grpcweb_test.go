package compatibility_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/i2y/hyperway/rpc"
)

// TestSimpleGRPCWeb tests basic gRPC-Web handling
func TestSimpleGRPCWeb(t *testing.T) {
	// Create a simple service
	svc := rpc.NewService("TestService", rpc.WithPackage("test.v1"))

	// Simple types for testing
	type EchoReq struct {
		Message string `json:"message"`
	}
	type EchoResp struct {
		Message string `json:"message"`
	}

	// Register handler
	err := rpc.Register(svc, "Echo", func(ctx context.Context, req *EchoReq) (*EchoResp, error) {
		fmt.Printf("Handler called with: %+v\n", req)
		return &EchoResp{Message: "Echo: " + req.Message}, nil
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Create handler
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Start server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test sending a framed gRPC-Web+JSON request manually
	t.Run("Manual gRPC-Web+JSON", func(t *testing.T) {
		// Create JSON payload
		jsonData, _ := json.Marshal(map[string]string{"message": "test"})

		// Create frame (5 bytes header + JSON)
		frame := make([]byte, 5+len(jsonData))
		frame[0] = 0 // No compression flag
		binary.BigEndian.PutUint32(frame[1:5], uint32(len(jsonData)))
		copy(frame[5:], jsonData)

		// Send request
		req, _ := http.NewRequest("POST", server.URL+"/test.v1.TestService/Echo", bytes.NewReader(frame))
		req.Header.Set("Content-Type", "application/grpc-web+json")

		fmt.Printf("Sending %d bytes to %s\n", len(frame), req.URL)
		fmt.Printf("Frame header: %02x %02x %02x %02x %02x\n", frame[0], frame[1], frame[2], frame[3], frame[4])

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Read response
		body, _ := io.ReadAll(resp.Body)

		fmt.Printf("Response status: %d\n", resp.StatusCode)
		fmt.Printf("Response Content-Type: %s\n", resp.Header.Get("Content-Type"))
		fmt.Printf("Response body (%d bytes): %x\n", len(body), body)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Parse response frame if present
		if len(body) >= 5 {
			respFlag := body[0]
			respLen := binary.BigEndian.Uint32(body[1:5])
			fmt.Printf("Response frame: flag=%02x, length=%d\n", respFlag, respLen)

			if int(respLen) <= len(body)-5 {
				respJSON := body[5 : 5+respLen]
				var respData map[string]interface{}
				if err := json.Unmarshal(respJSON, &respData); err == nil {
					fmt.Printf("Response JSON: %+v\n", respData)
				}
			}
		}
	})
}
