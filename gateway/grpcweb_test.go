package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCWebFraming(t *testing.T) {
	tests := []struct {
		name    string
		mode    grpcWebMode
		message []byte
	}{
		{
			name:    "binary mode small message",
			mode:    grpcWebModeBinary,
			message: []byte("hello world"),
		},
		{
			name:    "binary mode empty message",
			mode:    grpcWebModeBinary,
			message: []byte{},
		},
		{
			name:    "base64 mode small message",
			mode:    grpcWebModeBase64,
			message: []byte("hello world"),
		},
		{
			name:    "base64 mode large message",
			mode:    grpcWebModeBase64,
			message: bytes.Repeat([]byte("test"), 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write frame
			var buf bytes.Buffer
			writer := newGRPCWebFrameWriter(&buf, tt.mode)

			err := writer.writeDataFrame(tt.message)
			if err != nil {
				t.Fatalf("writeDataFrame failed: %v", err)
			}

			// Write trailer
			trailers := []byte("grpc-status: 0\r\n")
			err = writer.writeTrailerFrame(trailers)
			if err != nil {
				t.Fatalf("writeTrailerFrame failed: %v", err)
			}

			writer.close()

			// Read frames back
			reader := newGRPCWebFrameReader(&buf, tt.mode)

			// Read data frame
			frame, err := reader.readFrame()
			if err != nil {
				t.Fatalf("readFrame failed: %v", err)
			}

			if frame.flag != grpcWebMessageFlagData {
				t.Errorf("expected data flag, got %x", frame.flag)
			}

			if !bytes.Equal(frame.payload, tt.message) {
				t.Errorf("payload mismatch: got %q, want %q", frame.payload, tt.message)
			}

			// Read trailer frame
			frame, err = reader.readFrame()
			if err != nil {
				t.Fatalf("readFrame failed: %v", err)
			}

			if frame.flag != grpcWebMessageFlagTrailer {
				t.Errorf("expected trailer flag, got %x", frame.flag)
			}

			if !bytes.Equal(frame.payload, trailers) {
				t.Errorf("trailer mismatch: got %q, want %q", frame.payload, trailers)
			}
		})
	}
}

func TestDetectGRPCWebMode(t *testing.T) {
	tests := []struct {
		contentType string
		expected    grpcWebMode
	}{
		{"application/grpc-web", grpcWebModeBinary},
		{"application/grpc-web+proto", grpcWebModeBinary},
		{"application/grpc-web-text", grpcWebModeBase64},
		{"application/grpc-web-text+proto", grpcWebModeBase64},
		{"application/grpc", grpcWebModeBinary}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			mode := detectGRPCWebMode(tt.contentType)
			if mode != tt.expected {
				t.Errorf("detectGRPCWebMode(%q) = %v, want %v", tt.contentType, mode, tt.expected)
			}
		})
	}
}

func TestParseTrailerFrame(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected map[string]string
	}{
		{
			name:    "single header",
			payload: "grpc-status: 0",
			expected: map[string]string{
				"grpc-status": "0",
			},
		},
		{
			name:    "multiple headers",
			payload: "grpc-status: 0\r\ngrpc-message: OK\r\ncustom-header: value",
			expected: map[string]string{
				"grpc-status":   "0",
				"grpc-message":  "OK",
				"custom-header": "value",
			},
		},
		{
			name:    "headers with spaces",
			payload: "grpc-status: 0\r\ngrpc-message: Error message with spaces",
			expected: map[string]string{
				"grpc-status":  "0",
				"grpc-message": "Error message with spaces",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := parseTrailerFrame([]byte(tt.payload))

			for key, expectedValue := range tt.expected {
				values := headers.Values(key)
				if len(values) != 1 || values[0] != expectedValue {
					t.Errorf("header %q = %v, want %q", key, values, expectedValue)
				}
			}
		})
	}
}

func TestIsGRPCWeb(t *testing.T) {
	tests := []struct {
		name          string
		contentType   string
		grpcWebHeader string
		expected      bool
	}{
		{
			name:        "grpc-web content type",
			contentType: "application/grpc-web+proto",
			expected:    true,
		},
		{
			name:        "grpc-web-text content type",
			contentType: "application/grpc-web-text",
			expected:    true,
		},
		{
			name:          "x-grpc-web header",
			contentType:   "application/grpc",
			grpcWebHeader: "1",
			expected:      true,
		},
		{
			name:        "regular grpc",
			contentType: "application/grpc+proto",
			expected:    false,
		},
		{
			name:        "json request",
			contentType: "application/json",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", http.NoBody)
			req.Header.Set("Content-Type", tt.contentType)
			if tt.grpcWebHeader != "" {
				req.Header.Set("X-Grpc-Web", tt.grpcWebHeader)
			}

			result := isGRPCWeb(req)
			if result != tt.expected {
				t.Errorf("isGRPCWeb() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGRPCWebHandler(t *testing.T) {
	// Create a mock gRPC handler
	grpcHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request transformation
		if ct := r.Header.Get("Content-Type"); ct != "application/protobuf" {
			t.Errorf("expected protobuf content type, got %q", ct)
		}

		// Read request body (should be raw protobuf, not gRPC frame)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}

		// Verify we got the expected data
		// The actual data varies by test case, so just verify we got something
		if len(body) == 0 {
			t.Errorf("expected non-empty request body")
		}

		// Write response
		w.Header().Set("grpc-status", "0")
		w.Header().Set("grpc-message", "OK")
		w.Write([]byte("response data"))
	})

	// Create gRPC-Web handler
	handler := newGRPCWebHandler(grpcHandler, 0)

	tests := []struct {
		name        string
		contentType string
		requestData []byte
	}{
		{
			name:        "binary mode",
			contentType: "application/grpc-web",
			requestData: []byte("test request"),
		},
		{
			name:        "base64 mode",
			contentType: "application/grpc-web-text",
			requestData: []byte("test request base64"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body with proper framing
			var requestBuf bytes.Buffer
			codec := newGRPCWebCodec(tt.contentType)
			writer := newGRPCWebFrameWriter(&requestBuf, codec.mode)

			err := writer.writeDataFrame(tt.requestData)
			if err != nil {
				t.Fatalf("failed to write request frame: %v", err)
			}
			writer.close()

			// Create HTTP request
			req := httptest.NewRequest("POST", "/test.Service/Method", &requestBuf)
			req.Header.Set("Content-Type", tt.contentType)

			// Record response
			rec := httptest.NewRecorder()

			// Handle request
			handler.ServeHTTP(rec, req)

			// Verify response content type
			respContentType := rec.Header().Get("Content-Type")
			if !strings.Contains(respContentType, "grpc-web") {
				t.Errorf("expected grpc-web content type, got %q", respContentType)
			}

			// Read response frames
			respReader := newGRPCWebFrameReader(rec.Body, codec.mode)

			// Read data frame
			frame, err := respReader.readFrame()
			if err != nil {
				t.Fatalf("failed to read response frame: %v", err)
			}

			if frame.flag != grpcWebMessageFlagData {
				t.Errorf("expected data frame, got flag %x", frame.flag)
			}

			if string(frame.payload) != "response data" {
				t.Errorf("unexpected response data: %q", frame.payload)
			}

			// Read trailer frame
			frame, err = respReader.readFrame()
			if err != nil {
				t.Fatalf("failed to read trailer frame: %v", err)
			}

			if frame.flag != grpcWebMessageFlagTrailer {
				t.Errorf("expected trailer frame, got flag %x", frame.flag)
			}

			// Parse trailers
			trailers := parseTrailerFrame(frame.payload)

			if status := trailers.Get("grpc-status"); status != "0" {
				t.Errorf("expected status 0, got %q", status)
			}

			if message := trailers.Get("grpc-message"); message != "OK" {
				t.Errorf("expected message OK, got %q", message)
			}
		})
	}
}

func TestGRPCWebErrorHandling(t *testing.T) {
	// Create a handler that returns an error
	grpcHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("grpc-status", strconv.Itoa(int(codes.NotFound)))
		w.Header().Set("grpc-message", "resource not found")
		// Don't write body for error response
	})

	handler := newGRPCWebHandler(grpcHandler, 0)

	// Create request
	var requestBuf bytes.Buffer
	writer := newGRPCWebFrameWriter(&requestBuf, grpcWebModeBinary)
	writer.writeDataFrame([]byte("test"))
	writer.close()

	req := httptest.NewRequest("POST", "/test", &requestBuf)
	req.Header.Set("Content-Type", "application/grpc-web")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Read response
	reader := newGRPCWebFrameReader(rec.Body, grpcWebModeBinary)

	// Should have empty data frame
	frame, err := reader.readFrame()
	if err != nil {
		t.Fatalf("failed to read data frame: %v", err)
	}
	if len(frame.payload) > 0 {
		t.Errorf("expected empty data frame for error")
	}

	// Read trailer with error
	frame, err = reader.readFrame()
	if err != nil {
		t.Fatalf("failed to read trailer frame: %v", err)
	}

	trailers := parseTrailerFrame(frame.payload)
	if status := trailers.Get("grpc-status"); status != strconv.Itoa(int(codes.NotFound)) {
		t.Errorf("expected NotFound status, got %q", status)
	}
}

func TestGRPCWebBase64Encoding(t *testing.T) {
	message := []byte("Hello, gRPC-Web!")

	// Test base64 encoding
	var buf bytes.Buffer
	writer := newGRPCWebFrameWriter(&buf, grpcWebModeBase64)

	err := writer.writeDataFrame(message)
	if err != nil {
		t.Fatalf("writeDataFrame failed: %v", err)
	}
	writer.close()

	// The output should be base64 encoded
	encoded := buf.String()

	// Decode and verify
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	// Check frame structure
	if len(decoded) < grpcWebFrameHeaderSize {
		t.Fatalf("decoded data too short")
	}

	flag := decoded[0]
	length := binary.BigEndian.Uint32(decoded[1:5])
	payload := decoded[5:]

	if flag != grpcWebMessageFlagData {
		t.Errorf("expected data flag, got %x", flag)
	}

	if int(length) != len(message) {
		t.Errorf("length mismatch: got %d, want %d", length, len(message))
	}

	if !bytes.Equal(payload, message) {
		t.Errorf("payload mismatch: got %q, want %q", payload, message)
	}
}

func TestExtractMetadata(t *testing.T) {
	headers := http.Header{
		"Authorization":   []string{"Bearer token"},
		"X-Custom-Header": []string{"value1", "value2"},
		"Content-Type":    []string{"application/grpc-web"},
		"X-Grpc-Web":      []string{"1"},
		"User-Agent":      []string{"test-client"},
		"x-lowercase":     []string{"should be included"},
	}

	md := extractMetadata(headers)

	// Should include custom headers
	if auth := md.Get("authorization"); len(auth) != 1 || auth[0] != "Bearer token" {
		t.Errorf("authorization header not extracted correctly: %v", auth)
	}

	if custom := md.Get("x-custom-header"); len(custom) != 2 {
		t.Errorf("custom header not extracted correctly: %v", custom)
	}

	if lower := md.Get("x-lowercase"); len(lower) != 1 || lower[0] != "should be included" {
		t.Errorf("lowercase header not extracted correctly: %v", lower)
	}

	// Should exclude system headers
	if ct := md.Get("content-type"); len(ct) > 0 {
		t.Errorf("content-type should not be in metadata: %v", ct)
	}

	if xgw := md.Get("x-grpc-web"); len(xgw) > 0 {
		t.Errorf("x-grpc-web should not be in metadata: %v", xgw)
	}
}

func TestGRPCWebInvalidRequests(t *testing.T) {
	handler := newGRPCWebHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid requests")
	}), 0)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT request",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", http.NoBody)
			req.Header.Set("Content-Type", "application/grpc-web")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestFormatTrailerFrame(t *testing.T) {
	headers := http.Header{
		"grpc-status":    []string{"0"},
		"grpc-message":   []string{"Everything is OK"},
		"custom-trailer": []string{"value1", "value2"},
	}

	formatted := formatTrailerFrame(headers)
	formattedStr := string(formatted)

	// Should contain all headers
	if !strings.Contains(formattedStr, "grpc-status: 0") {
		t.Error("missing grpc-status in formatted trailers")
	}

	if !strings.Contains(formattedStr, "grpc-message: Everything is OK") {
		t.Error("missing grpc-message in formatted trailers")
	}

	// Should handle multiple values
	if !strings.Contains(formattedStr, "custom-trailer: value1") ||
		!strings.Contains(formattedStr, "custom-trailer: value2") {
		t.Error("missing custom trailer values")
	}

	// Should use CRLF line endings
	if !strings.Contains(formattedStr, "\r\n") {
		t.Error("trailers should use CRLF line endings")
	}
}

func TestGRPCWebInterceptor(t *testing.T) {
	// Create a base handler
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should be called for non-gRPC-Web requests
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("regular response"))
	})

	// Create interceptor
	interceptor := NewGRPCWebInterceptor(baseHandler, 0)

	t.Run("gRPC-Web request", func(t *testing.T) {
		// Create gRPC-Web request
		var buf bytes.Buffer
		writer := newGRPCWebFrameWriter(&buf, grpcWebModeBinary)
		writer.writeDataFrame([]byte("test"))
		writer.close()

		req := httptest.NewRequest("POST", "/test", &buf)
		req.Header.Set("Content-Type", "application/grpc-web")

		rec := httptest.NewRecorder()
		interceptor.ServeHTTP(rec, req)

		// Should get gRPC-Web response
		if !strings.Contains(rec.Header().Get("Content-Type"), "grpc-web") {
			t.Error("expected gRPC-Web response")
		}
	})

	t.Run("regular request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader("test"))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		interceptor.ServeHTTP(rec, req)

		// Should get regular response
		if rec.Body.String() != "regular response" {
			t.Errorf("expected regular response, got %q", rec.Body.String())
		}
	})
}

func TestResponseRecorder(t *testing.T) {
	rec := newResponseRecorder()

	// Test header operations
	rec.Header().Set("Test-Header", "value")
	if v := rec.Header().Get("Test-Header"); v != "value" {
		t.Errorf("header not set correctly: %q", v)
	}

	// Test status code
	rec.WriteHeader(http.StatusCreated)
	if rec.status != http.StatusCreated {
		t.Errorf("status not set correctly: %d", rec.status)
	}

	// Test body write
	n, err := rec.Write([]byte("test body"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != 9 {
		t.Errorf("wrong number of bytes written: %d", n)
	}
	if rec.body.String() != "test body" {
		t.Errorf("body not written correctly: %q", rec.body.String())
	}
}

func TestGRPCWebCodec(t *testing.T) {
	tests := []struct {
		contentType  string
		expectedMode grpcWebMode
		expectedCT   string
	}{
		{
			contentType:  "application/grpc-web",
			expectedMode: grpcWebModeBinary,
			expectedCT:   "application/grpc-web+proto",
		},
		{
			contentType:  "application/grpc-web-text",
			expectedMode: grpcWebModeBase64,
			expectedCT:   "application/grpc-web-text+proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			codec := newGRPCWebCodec(tt.contentType)

			if codec.mode != tt.expectedMode {
				t.Errorf("wrong mode: got %v, want %v", codec.mode, tt.expectedMode)
			}

			if codec.isBase64Mode() != (tt.expectedMode == grpcWebModeBase64) {
				t.Error("isBase64Mode() returned wrong value")
			}

			if ct := codec.contentType(); ct != tt.expectedCT {
				t.Errorf("wrong content type: got %q, want %q", ct, tt.expectedCT)
			}
		})
	}
}

// TestConcurrentFrameWriting tests thread safety of frame writing
func TestConcurrentFrameWriting(t *testing.T) {
	var buf bytes.Buffer
	writer := newGRPCWebFrameWriter(&buf, grpcWebModeBinary)

	// This test just ensures no panics occur with concurrent writes
	// In practice, the HTTP handler should serialize access
	done := make(chan bool, 3)

	go func() {
		writer.writeDataFrame([]byte("message1"))
		done <- true
	}()

	go func() {
		writer.writeDataFrame([]byte("message2"))
		done <- true
	}()

	go func() {
		writer.writeTrailerFrame([]byte("grpc-status: 0\r\n"))
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Just verify we have some output and no panic occurred
	if buf.Len() == 0 {
		t.Error("no data written")
	}
}

func TestErrorToStatus(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
		expectedMsg  string
	}{
		{
			name:         "grpc status error",
			err:          status.Error(codes.NotFound, "not found"),
			expectedCode: codes.NotFound,
			expectedMsg:  "not found",
		},
		{
			name:         "regular error",
			err:          io.EOF,
			expectedCode: codes.Internal,
			expectedMsg:  "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &grpcWebHandler{}

			var buf bytes.Buffer
			writer := newGRPCWebFrameWriter(&buf, grpcWebModeBinary)

			handler.writeErrorResponse(writer, tt.err)

			// Read response
			reader := newGRPCWebFrameReader(&buf, grpcWebModeBinary)

			// Should have empty data frame
			frame, _ := reader.readFrame()
			if len(frame.payload) != 0 {
				t.Error("expected empty data frame")
			}

			// Check trailer
			frame, _ = reader.readFrame()
			trailers := parseTrailerFrame(frame.payload)

			if status := trailers.Get("grpc-status"); status != strconv.Itoa(int(tt.expectedCode)) {
				t.Errorf("wrong status code: got %s, want %d", status, tt.expectedCode)
			}

			if msg := trailers.Get("grpc-message"); msg != tt.expectedMsg {
				t.Errorf("wrong message: got %q, want %q", msg, tt.expectedMsg)
			}
		})
	}
}
