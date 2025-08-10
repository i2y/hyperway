package compatibility_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	compatibility "github.com/i2y/hyperway/compatibility-tests"
	"github.com/klauspost/compress/zstd"
)

// TestCompressionCompatibility tests gzip compression between Hyperway and Connect-go
func TestCompressionCompatibility(t *testing.T) {
	// Start Hyperway server
	handler, err := compatibility.CreateHyperwayServer()
	if err != nil {
		t.Fatalf("Failed to create Hyperway server: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("uncompressed_request_compressed_response", func(t *testing.T) {
		// Large payload to trigger compression (>1KB)
		largeString := strings.Repeat("Hello World! ", 100)
		req := compatibility.SimpleRequest{
			StringField: largeString,
			Int32Field:  42,
			BoolField:   true,
		}

		// Test with Connect protocol
		resp, respHeaders, err := callWithCompression(server.URL, "connect", &req, false, true)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Check if response was compressed
		if encoding := respHeaders.Get("Content-Encoding"); encoding != "gzip" {
			t.Errorf("Expected compressed response, got Content-Encoding: %s", encoding)
		}

		// Verify response data
		if resp.StringField != largeString {
			t.Errorf("Response string mismatch: got %d chars, want %d chars",
				len(resp.StringField), len(largeString))
		}
	})

	t.Run("compressed_request_compressed_response", func(t *testing.T) {
		// Large payload to trigger compression
		largeString := strings.Repeat("Compressed data test! ", 100)
		req := compatibility.SimpleRequest{
			StringField: largeString,
			Int32Field:  999,
			DoubleField: 3.14159,
			BytesField:  []byte(strings.Repeat("binary", 50)),
		}

		// Test with compressed request and expect compressed response
		resp, respHeaders, err := callWithCompression(server.URL, "connect", &req, true, true)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Check if response was compressed
		if encoding := respHeaders.Get("Content-Encoding"); encoding != "gzip" {
			t.Errorf("Expected compressed response, got Content-Encoding: %s", encoding)
		}

		// Verify response data integrity
		if resp.StringField != largeString {
			t.Errorf("Response string mismatch after compression")
		}
		if resp.Int32Field != 999 {
			t.Errorf("Response int32 mismatch: got %d, want 999", resp.Int32Field)
		}
	})

	t.Run("small_payload_no_compression", func(t *testing.T) {
		// Small payload (< 1KB) should not trigger compression
		req := compatibility.SimpleRequest{
			StringField: "small",
			Int32Field:  1,
		}

		resp, respHeaders, err := callWithCompression(server.URL, "connect", &req, false, true)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Check that response was NOT compressed (small payload)
		if encoding := respHeaders.Get("Content-Encoding"); encoding == "gzip" {
			t.Errorf("Small payload should not be compressed, but got Content-Encoding: %s", encoding)
		}

		// Verify response
		if resp.StringField != "small" {
			t.Errorf("Response mismatch: got %s, want 'small'", resp.StringField)
		}
	})

	t.Run("grpc_compression", func(t *testing.T) {
		// Test gRPC protocol with compression
		largeString := strings.Repeat("gRPC compression test ", 100)
		req := compatibility.SimpleRequest{
			StringField: largeString,
			Int32Field:  42,
		}

		resp, err := callGRPCWithCompression(server.URL, &req, true)
		if err != nil {
			t.Fatalf("gRPC request failed: %v", err)
		}

		// Verify response
		if resp.StringField != largeString {
			t.Errorf("gRPC response mismatch: got %d chars, want %d chars",
				len(resp.StringField), len(largeString))
		}
	})

	t.Run("streaming_with_compression", func(t *testing.T) {
		// Test streaming with compression
		testStreamingWithCompression(t, server.URL)
	})

	t.Run("grpc_streaming_with_compression", func(t *testing.T) {
		// Test gRPC streaming with compression
		testGRPCStreamingWithCompression(t, server.URL)
	})

	t.Run("brotli_compression", func(t *testing.T) {
		// Test with Brotli compression
		testBrotliCompression(t, server.URL)
	})

	t.Run("zstd_compression", func(t *testing.T) {
		// Test with Zstandard compression
		testZstdCompression(t, server.URL)
	})
}

// Helper function to make requests with compression options
func callWithCompression(baseURL, protocol string, req *compatibility.SimpleRequest, compressRequest, acceptCompressed bool) (*compatibility.SimpleResponse, http.Header, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/SimpleEcho", baseURL)

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	// Compress request body if needed
	var body []byte
	contentEncoding := ""
	if compressRequest && len(reqBody) > 100 { // Only compress if worthwhile
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(reqBody); err != nil {
			return nil, nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, nil, err
		}
		body = buf.Bytes()
		contentEncoding = "gzip"
	} else {
		body = reqBody
	}

	// Create request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")

	if contentEncoding != "" {
		httpReq.Header.Set("Content-Encoding", contentEncoding)
	}

	if acceptCompressed {
		httpReq.Header.Set("Accept-Encoding", "gzip")
		httpReq.Header.Set("Connect-Accept-Encoding", "gzip")
	}

	// Make request
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Decompress response if needed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(bytes.NewReader(respBody))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gz.Close()

		respBody, err = io.ReadAll(gz)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decompress response: %v", err)
		}
	}

	// Unmarshal response
	var result compatibility.SimpleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, nil, err
	}

	return &result, resp.Header, nil
}

// Helper for gRPC with compression
func callGRPCWithCompression(baseURL string, req *compatibility.SimpleRequest, useCompression bool) (*compatibility.SimpleResponse, error) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/SimpleEcho", baseURL)

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Prepare gRPC frame
	var frameData []byte
	var frameHeader [5]byte

	if useCompression && len(reqBody) > 100 {
		// Compress the message
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(reqBody); err != nil {
			return nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, err
		}

		compressed := buf.Bytes()
		frameHeader[0] = 1 // Set compression flag
		msgLen := len(compressed)
		frameHeader[1] = byte(msgLen >> 24)
		frameHeader[2] = byte(msgLen >> 16)
		frameHeader[3] = byte(msgLen >> 8)
		frameHeader[4] = byte(msgLen)
		frameData = append(frameHeader[:], compressed...)
	} else {
		// No compression
		frameHeader[0] = 0
		msgLen := len(reqBody)
		frameHeader[1] = byte(msgLen >> 24)
		frameHeader[2] = byte(msgLen >> 16)
		frameHeader[3] = byte(msgLen >> 8)
		frameHeader[4] = byte(msgLen)
		frameData = append(frameHeader[:], reqBody...)
	}

	// Create request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(frameData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/grpc+json")
	if useCompression {
		httpReq.Header.Set("grpc-encoding", "gzip")
		httpReq.Header.Set("grpc-accept-encoding", "gzip")
	}

	// Make request
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	// Read gRPC response frame
	respFrame := make([]byte, 5)
	if _, err := io.ReadFull(resp.Body, respFrame); err != nil {
		return nil, fmt.Errorf("failed to read response frame: %v", err)
	}

	// Parse frame header
	compressed := respFrame[0] == 1
	msgLen := int(respFrame[1])<<24 | int(respFrame[2])<<16 | int(respFrame[3])<<8 | int(respFrame[4])

	// Read message
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(resp.Body, msgData); err != nil {
		return nil, fmt.Errorf("failed to read response message: %v", err)
	}

	// Decompress if needed
	if compressed {
		gz, err := gzip.NewReader(bytes.NewReader(msgData))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gz.Close()

		msgData, err = io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress message: %v", err)
		}
	}

	// Unmarshal response
	var result compatibility.SimpleResponse
	if err := json.Unmarshal(msgData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Test streaming with compression
func testStreamingWithCompression(t *testing.T, baseURL string) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/ServerStreamLarge", baseURL)

	// Request large messages that will trigger compression
	req := compatibility.StreamRequest{
		Count: 5,
	}
	reqBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Connect-Accept-Encoding", "gzip")
	httpReq.Header.Set("Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s", resp.StatusCode, body)
	}

	// Check if response indicates compression support
	if encoding := resp.Header.Get("Connect-Accept-Encoding"); encoding != "" {
		t.Logf("Server accepts encoding: %s", encoding)
	}

	// Read streaming responses
	reader := resp.Body
	count := 0
	compressedCount := 0

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

		// Check compression flag
		isCompressed := (header[0] & 0x01) != 0
		if isCompressed {
			compressedCount++
		}

		// Parse message length
		msgLen := int(header[1])<<24 | int(header[2])<<16 | int(header[3])<<8 | int(header[4])

		// Check for end-of-stream
		if header[0]&0x01 != 0 && msgLen == 0 {
			break
		}

		// Read message body
		msgBody := make([]byte, msgLen)
		if _, err := io.ReadFull(reader, msgBody); err != nil {
			t.Logf("Failed to read message body: %v", err)
			break
		}

		// Skip empty messages
		if msgLen == 2 && string(msgBody) == "{}" {
			continue
		}

		// Decompress if needed
		if isCompressed {
			gz, err := gzip.NewReader(bytes.NewReader(msgBody))
			if err != nil {
				t.Logf("Failed to create gzip reader: %v", err)
				continue
			}
			defer gz.Close()

			msgBody, err = io.ReadAll(gz)
			if err != nil {
				t.Logf("Failed to decompress: %v", err)
				continue
			}
		}

		// Decode message
		var msg compatibility.StreamResponse
		if err := json.Unmarshal(msgBody, &msg); err != nil {
			t.Logf("Failed to decode message: %v", err)
			continue
		}

		count++
	}

	if count == 0 {
		t.Error("No streaming responses received")
	} else {
		t.Logf("Received %d streaming responses, %d were compressed", count, compressedCount)
	}
}

// Test gRPC streaming with compression
func testGRPCStreamingWithCompression(t *testing.T, baseURL string) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/ServerStreamLarge", baseURL)

	// Create request
	req := compatibility.StreamRequest{Count: 3}
	reqBody, _ := json.Marshal(req)

	// Prepare gRPC frame for request
	var frameHeader [5]byte
	frameHeader[0] = 0 // no compression for request
	msgLen := len(reqBody)
	frameHeader[1] = byte(msgLen >> 24)
	frameHeader[2] = byte(msgLen >> 16)
	frameHeader[3] = byte(msgLen >> 8)
	frameHeader[4] = byte(msgLen)
	frameData := append(frameHeader[:], reqBody...)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(frameData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/grpc+json")
	httpReq.Header.Set("grpc-accept-encoding", "gzip")
	httpReq.Header.Set("te", "trailers") // gRPC requires TE: trailers header

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s\nHeaders: %v", resp.StatusCode, body, resp.Header)
	}

	// Log all response headers for debugging
	t.Logf("Response headers: %v", resp.Header)

	// Check if server indicates compression support
	if encoding := resp.Header.Get("grpc-encoding"); encoding != "gzip" {
		t.Logf("Server does not indicate gzip encoding in headers: %s", encoding)
	}

	// Read streaming responses
	reader := resp.Body
	count := 0
	compressedCount := 0

	for {
		// Read 5-byte frame header
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF {
				break
			}
			// This might be trailers
			break
		}

		// Check compression flag
		isCompressed := (header[0] & 0x01) != 0
		if isCompressed {
			compressedCount++
		}

		// Parse message length
		msgLen := int(header[1])<<24 | int(header[2])<<16 | int(header[3])<<8 | int(header[4])

		// Read message body
		msgBody := make([]byte, msgLen)
		if _, err := io.ReadFull(reader, msgBody); err != nil {
			t.Logf("Failed to read message body: %v", err)
			break
		}

		// Decompress if needed
		if isCompressed {
			gz, err := gzip.NewReader(bytes.NewReader(msgBody))
			if err != nil {
				t.Logf("Failed to create gzip reader: %v", err)
				continue
			}
			defer gz.Close()

			msgBody, err = io.ReadAll(gz)
			if err != nil {
				t.Logf("Failed to decompress: %v", err)
				continue
			}
		}

		// Decode message
		var msg compatibility.StreamResponse
		if err := json.Unmarshal(msgBody, &msg); err != nil {
			t.Logf("Failed to decode message: %v", err)
			continue
		}

		count++
	}

	if count == 0 {
		t.Error("No gRPC streaming responses received")
	} else {
		t.Logf("Received %d gRPC streaming responses, %d were compressed", count, compressedCount)
		if compressedCount == 0 {
			t.Error("Expected compressed messages but none were compressed")
		}
	}
}

// Test Brotli compression
func testBrotliCompression(t *testing.T, baseURL string) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/SimpleEcho", baseURL)

	// Large payload to trigger compression
	largeString := strings.Repeat("Brotli compression test! ", 100)
	req := compatibility.SimpleRequest{
		StringField: largeString,
		Int32Field:  99,
	}

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Accept-Encoding", "br") // Request Brotli compression
	httpReq.Header.Set("Connect-Accept-Encoding", "br")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s", resp.StatusCode, body)
	}

	// Check if response was compressed with Brotli
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "br" {
		t.Errorf("Expected Brotli compressed response, got Content-Encoding: %s", encoding)
	}

	// Decompress if needed
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Header.Get("Content-Encoding") == "br" {
		reader := brotli.NewReader(bytes.NewReader(respBody))
		respBody, err = io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to decompress Brotli: %v", err)
		}
	}

	// Verify response
	var result compatibility.SimpleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.StringField != largeString {
		t.Errorf("Response mismatch: got %d chars, want %d chars",
			len(result.StringField), len(largeString))
	}

	t.Logf("Brotli compression successful")
}

// Test Zstandard compression
func testZstdCompression(t *testing.T, baseURL string) {
	url := fmt.Sprintf("%s/compatibility.v1.CompatibilityService/SimpleEcho", baseURL)

	// Large payload to trigger compression
	largeString := strings.Repeat("Zstandard compression test! ", 100)
	req := compatibility.SimpleRequest{
		StringField: largeString,
		Int32Field:  77,
	}

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Accept-Encoding", "zstd") // Request Zstd compression
	httpReq.Header.Set("Connect-Accept-Encoding", "zstd")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s", resp.StatusCode, body)
	}

	// Check if response was compressed with Zstd
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "zstd" {
		t.Errorf("Expected Zstd compressed response, got Content-Encoding: %s", encoding)
	}

	// Decompress if needed
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Header.Get("Content-Encoding") == "zstd" {
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			t.Fatalf("Failed to create Zstd decoder: %v", err)
		}
		defer decoder.Close()

		respBody, err = decoder.DecodeAll(respBody, nil)
		if err != nil {
			t.Fatalf("Failed to decompress Zstd: %v", err)
		}
	}

	// Verify response
	var result compatibility.SimpleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.StringField != largeString {
		t.Errorf("Response mismatch: got %d chars, want %d chars",
			len(result.StringField), len(largeString))
	}

	t.Logf("Zstandard compression successful")
}
