package compatibility_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	compatibility "github.com/i2y/hyperway/compatibility-tests"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestSimpleDataTypes tests basic scalar type compatibility
func TestSimpleDataTypes(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name string
		req  compatibility.SimpleRequest
	}{
		{
			name: "all scalar types",
			req: compatibility.SimpleRequest{
				StringField: "test string",
				Int32Field:  42,
				Int64Field:  9876543210,
				FloatField:  3.14,
				DoubleField: 2.71828,
				BoolField:   true,
				BytesField:  []byte("test bytes"),
			},
		},
		{
			name: "empty values",
			req: compatibility.SimpleRequest{
				StringField: "",
				Int32Field:  0,
				Int64Field:  0,
				FloatField:  0,
				DoubleField: 0,
				BoolField:   false,
				BytesField:  nil,
			},
		},
		{
			name: "negative numbers",
			req: compatibility.SimpleRequest{
				StringField: "negative",
				Int32Field:  -42,
				Int64Field:  -9876543210,
				FloatField:  -3.14,
				DoubleField: -2.71828,
				BoolField:   false,
				BytesField:  []byte{0xFF, 0x00, 0xAB},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with Connect protocol
			t.Run("connect_protocol", func(t *testing.T) {
				resp, err := callSimpleEcho(server.URL, "connect", &tt.req)
				if err != nil {
					t.Fatalf("Connect protocol call failed: %v", err)
				}
				assertSimpleResponse(t, &tt.req, resp)
			})

			// Test with gRPC protocol (using Connect client in gRPC mode)
			t.Run("grpc_protocol", func(t *testing.T) {
				resp, err := callSimpleEcho(server.URL, "grpc", &tt.req)
				if err != nil {
					t.Fatalf("gRPC protocol call failed: %v", err)
				}
				assertSimpleResponse(t, &tt.req, resp)
			})

			// Test with gRPC-Web protocol
			t.Run("grpc_web_protocol", func(t *testing.T) {
				resp, err := callSimpleEcho(server.URL, "grpcweb", &tt.req)
				if err != nil {
					t.Fatalf("gRPC-Web protocol call failed: %v", err)
				}
				assertSimpleResponse(t, &tt.req, resp)
			})
		})
	}
}

// TestComplexDataTypes tests nested messages, maps, and repeated fields
func TestComplexDataTypes(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	optionalStr := "optional value"
	intChoice := int32(42)

	tests := []struct {
		name string
		req  compatibility.ComplexRequest
	}{
		{
			name: "full complex message",
			req: compatibility.ComplexRequest{
				NestedMessage: &compatibility.NestedMessage{
					ID:    "nested-1",
					Value: 100,
				},
				RepeatedField: []string{"one", "two", "three"},
				MapField: map[string]int32{
					"key1": 10,
					"key2": 20,
					"key3": 30,
				},
				OptionalField: &optionalStr,
				OneofField: &compatibility.OneofMessage{
					Choice: struct {
						StringChoice *string `json:"string_choice,omitempty"`
						IntChoice    *int32  `json:"int_choice,omitempty"`
						BoolChoice   *bool   `json:"bool_choice,omitempty"`
					}{
						IntChoice: &intChoice,
					},
				},
			},
		},
		{
			name: "empty complex message",
			req: compatibility.ComplexRequest{
				NestedMessage: nil,
				RepeatedField: nil,
				MapField:      nil,
				OptionalField: nil,
				OneofField:    nil,
			},
		},
		{
			name: "empty collections",
			req: compatibility.ComplexRequest{
				NestedMessage: &compatibility.NestedMessage{
					ID:    "",
					Value: 0,
				},
				RepeatedField: []string{},
				MapField:      map[string]int32{},
				OptionalField: nil,
				OneofField:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with Connect protocol
			resp, err := callComplexEcho(server.URL, "connect", &tt.req)
			if err != nil {
				t.Fatalf("Connect protocol call failed: %v", err)
			}
			assertComplexResponse(t, &tt.req, resp)
		})
	}
}

// TestWellKnownTypes tests Well-Known Types compatibility
func TestWellKnownTypes(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	now := time.Now()
	req := compatibility.WellKnownRequest{
		Timestamp:   timestamppb.New(now),
		Duration:    durationpb.New(5 * time.Minute),
		StringValue: wrapperspb.String("wrapped string"),
		Int32Value:  wrapperspb.Int32(42),
		BoolValue:   wrapperspb.Bool(true),
		EmptyValue:  &emptypb.Empty{},
	}

	// Test with Connect protocol
	resp, err := callWellKnownEcho(server.URL, "connect", &req)
	if err != nil {
		t.Fatalf("Connect protocol call failed: %v", err)
	}

	// Verify response
	if resp.Timestamp == nil || !resp.Timestamp.AsTime().Equal(now.Truncate(time.Microsecond)) {
		t.Errorf("Timestamp mismatch: got %v, want %v", resp.Timestamp, now)
	}
	if resp.Duration == nil || resp.Duration.AsDuration() != 5*time.Minute {
		t.Errorf("Duration mismatch: got %v, want %v", resp.Duration, 5*time.Minute)
	}
	if resp.StringValue == nil || resp.StringValue.Value != "wrapped string" {
		t.Errorf("StringValue mismatch: got %v, want 'wrapped string'", resp.StringValue)
	}
	if resp.Int32Value == nil || resp.Int32Value.Value != 42 {
		t.Errorf("Int32Value mismatch: got %v, want 42", resp.Int32Value)
	}
	if resp.BoolValue == nil || !resp.BoolValue.Value {
		t.Errorf("BoolValue mismatch: got %v, want true", resp.BoolValue)
	}
	if resp.EmptyValue == nil {
		t.Error("EmptyValue should not be nil")
	}
}

// TestServerStreaming tests server-streaming RPC compatibility
func TestServerStreaming(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req := compatibility.StreamRequest{Count: 5}

	// Make streaming request using Connect protocol
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/ServerStream", server.URL)

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Connect-Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s", resp.StatusCode, body)
	}

	// Read streaming responses
	// Connect streaming uses 5-byte envelope prefix for each message
	reader := resp.Body
	count := 0

	for {
		// Read 5-byte envelope header
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF {
				break
			}
			t.Logf("Failed to read envelope header: %v", err)
			break
		}

		// Parse message length from header
		// header[0] is flags (bit 0 = end-of-stream)
		// header[1-4] is big-endian message length
		msgLen := int(header[1])<<24 | int(header[2])<<16 | int(header[3])<<8 | int(header[4])

		// Check for end-of-stream flag
		if header[0]&0x01 != 0 && msgLen == 0 {
			// End of stream marker
			break
		}

		// Read message body
		msgBody := make([]byte, msgLen)
		if _, err := io.ReadFull(reader, msgBody); err != nil {
			t.Logf("Failed to read message body: %v", err)
			break
		}

		// Skip empty messages (likely trailers or end markers)
		if msgLen == 2 && string(msgBody) == "{}" {
			t.Logf("Skipping empty message (likely end-of-stream marker)")
			continue
		}

		// Decode JSON message
		var msg compatibility.StreamResponse
		if err := json.Unmarshal(msgBody, &msg); err != nil {
			t.Logf("Failed to decode message: %v, body: %s", err, msgBody)
			continue
		}

		t.Logf("Received message %d: %+v", count, msg)

		if msg.Index != int32(count) {
			t.Errorf("Expected index %d, got %d", count, msg.Index)
		}
		count++
	}

	if count == 0 {
		t.Error("No streaming responses received")
	} else {
		t.Logf("Successfully received %d streaming responses", count)
	}
}

// TestErrorHandling tests error code compatibility
func TestErrorHandling(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name      string
		errorCode string
		wantErr   bool
	}{
		{
			name:      "no error",
			errorCode: "",
			wantErr:   false,
		},
		{
			name:      "not found",
			errorCode: "not_found",
			wantErr:   true,
		},
		{
			name:      "invalid argument",
			errorCode: "invalid_argument",
			wantErr:   true,
		},
		{
			name:      "internal error",
			errorCode: "internal",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := compatibility.ErrorRequest{ErrorCode: tt.errorCode}

			resp, err := callTestError(server.URL, "connect", &req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none. Response: %+v", resp)
				} else {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil || !resp.Success {
					t.Error("Expected successful response")
				}
			}
		})
	}
}

// Helper functions for making RPC calls

func callSimpleEcho(baseURL, protocol string, req *compatibility.SimpleRequest) (*compatibility.SimpleResponse, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/SimpleEcho", baseURL)

	reqBody, _ := json.Marshal(req)

	// For gRPC protocols, we need to add framing
	var body []byte
	if protocol == "grpc" || protocol == "grpcweb" {
		// Add gRPC frame header (5 bytes: 1 flag + 4 length)
		frameHeader := make([]byte, 5)
		frameHeader[0] = 0 // no compression
		msgLen := len(reqBody)
		frameHeader[1] = byte(msgLen >> 24)
		frameHeader[2] = byte(msgLen >> 16)
		frameHeader[3] = byte(msgLen >> 8)
		frameHeader[4] = byte(msgLen)
		body = append(frameHeader, reqBody...)
	} else {
		body = reqBody
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	setProtocolHeaders(httpReq, protocol)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	// Read response based on protocol
	var result compatibility.SimpleResponse
	if protocol == "grpc" || protocol == "grpcweb" {
		// Read gRPC frame
		frameHeader := make([]byte, 5)
		if _, err := io.ReadFull(resp.Body, frameHeader); err != nil {
			return nil, fmt.Errorf("failed to read frame header: %v", err)
		}

		// Parse message length
		msgLen := int(frameHeader[1])<<24 | int(frameHeader[2])<<16 | int(frameHeader[3])<<8 | int(frameHeader[4])

		// Read message
		msgData := make([]byte, msgLen)
		if _, err := io.ReadFull(resp.Body, msgData); err != nil {
			return nil, fmt.Errorf("failed to read message: %v", err)
		}

		// For gRPC-Web, decode base64 if needed
		if protocol == "grpcweb" && resp.Header.Get("Content-Type") == "application/grpc-web+proto" {
			// Binary gRPC-Web, no additional decoding needed
		}

		// Decode JSON
		if err := json.Unmarshal(msgData, &result); err != nil {
			return nil, err
		}
	} else {
		// Connect protocol - direct JSON
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func callComplexEcho(baseURL, protocol string, req *compatibility.ComplexRequest) (*compatibility.ComplexResponse, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/ComplexEcho", baseURL)

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	setProtocolHeaders(httpReq, protocol)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	var result compatibility.ComplexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func callWellKnownEcho(baseURL, protocol string, req *compatibility.WellKnownRequest) (*compatibility.WellKnownResponse, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/WellKnownEcho", baseURL)

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	setProtocolHeaders(httpReq, protocol)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	var result compatibility.WellKnownResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func callTestError(baseURL, protocol string, req *compatibility.ErrorRequest) (*compatibility.ErrorResponse, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/TestError", baseURL)

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	setProtocolHeaders(httpReq, protocol)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// For Connect protocol, check for error in response
	if protocol == "connect" {
		if resp.StatusCode != http.StatusOK {
			// Connect protocol returns errors with status codes
			body, _ := io.ReadAll(resp.Body)
			var connectErr struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(body, &connectErr) == nil && connectErr.Code != "" {
				return nil, fmt.Errorf("%s: %s", connectErr.Code, connectErr.Message)
			}
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
		}
		// Even with 200 OK, Connect may return an error in the response body
		// This happens when the handler returns an error
	}

	// For gRPC, errors are in trailers
	if (protocol == "grpc" || protocol == "grpcweb") && resp.StatusCode == http.StatusOK {
		// Check grpc-status trailer
		grpcStatus := resp.Trailer.Get("grpc-status")
		if grpcStatus != "" && grpcStatus != "0" {
			grpcMessage := resp.Trailer.Get("grpc-message")
			return nil, fmt.Errorf("grpc error %s: %s", grpcStatus, grpcMessage)
		}
	}

	var result compatibility.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func setProtocolHeaders(req *http.Request, protocol string) {
	switch protocol {
	case "connect":
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
	case "grpc":
		req.Header.Set("Content-Type", "application/grpc+json")
	case "grpcweb":
		req.Header.Set("Content-Type", "application/grpc-web+json")
	default:
		// Default to Connect
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
	}
}

func assertSimpleResponse(t *testing.T, expected *compatibility.SimpleRequest, actual *compatibility.SimpleResponse) {
	t.Helper()

	if actual.StringField != expected.StringField {
		t.Errorf("StringField mismatch: got %q, want %q", actual.StringField, expected.StringField)
	}
	if actual.Int32Field != expected.Int32Field {
		t.Errorf("Int32Field mismatch: got %d, want %d", actual.Int32Field, expected.Int32Field)
	}
	if actual.Int64Field != expected.Int64Field {
		t.Errorf("Int64Field mismatch: got %d, want %d", actual.Int64Field, expected.Int64Field)
	}
	if actual.FloatField != expected.FloatField {
		t.Errorf("FloatField mismatch: got %f, want %f", actual.FloatField, expected.FloatField)
	}
	if actual.DoubleField != expected.DoubleField {
		t.Errorf("DoubleField mismatch: got %f, want %f", actual.DoubleField, expected.DoubleField)
	}
	if actual.BoolField != expected.BoolField {
		t.Errorf("BoolField mismatch: got %v, want %v", actual.BoolField, expected.BoolField)
	}
	if !bytes.Equal(actual.BytesField, expected.BytesField) {
		t.Errorf("BytesField mismatch: got %v, want %v", actual.BytesField, expected.BytesField)
	}
}

func assertComplexResponse(t *testing.T, expected *compatibility.ComplexRequest, actual *compatibility.ComplexResponse) {
	t.Helper()

	// Check nested message
	if expected.NestedMessage == nil {
		if actual.NestedMessage != nil {
			t.Error("NestedMessage should be nil")
		}
	} else {
		if actual.NestedMessage == nil {
			t.Error("NestedMessage should not be nil")
		} else {
			if actual.NestedMessage.ID != expected.NestedMessage.ID {
				t.Errorf("NestedMessage.ID mismatch: got %q, want %q",
					actual.NestedMessage.ID, expected.NestedMessage.ID)
			}
			if actual.NestedMessage.Value != expected.NestedMessage.Value {
				t.Errorf("NestedMessage.Value mismatch: got %d, want %d",
					actual.NestedMessage.Value, expected.NestedMessage.Value)
			}
		}
	}

	// Check repeated field
	if len(actual.RepeatedField) != len(expected.RepeatedField) {
		t.Errorf("RepeatedField length mismatch: got %d, want %d",
			len(actual.RepeatedField), len(expected.RepeatedField))
	} else {
		for i := range expected.RepeatedField {
			if actual.RepeatedField[i] != expected.RepeatedField[i] {
				t.Errorf("RepeatedField[%d] mismatch: got %q, want %q",
					i, actual.RepeatedField[i], expected.RepeatedField[i])
			}
		}
	}

	// Check map field
	if len(actual.MapField) != len(expected.MapField) {
		t.Errorf("MapField length mismatch: got %d, want %d",
			len(actual.MapField), len(expected.MapField))
	} else {
		for k, v := range expected.MapField {
			if actual.MapField[k] != v {
				t.Errorf("MapField[%q] mismatch: got %d, want %d", k, actual.MapField[k], v)
			}
		}
	}

	// Check optional field
	if expected.OptionalField == nil {
		if actual.OptionalField != nil {
			t.Error("OptionalField should be nil")
		}
	} else {
		if actual.OptionalField == nil {
			t.Error("OptionalField should not be nil")
		} else if *actual.OptionalField != *expected.OptionalField {
			t.Errorf("OptionalField mismatch: got %q, want %q",
				*actual.OptionalField, *expected.OptionalField)
		}
	}
}
