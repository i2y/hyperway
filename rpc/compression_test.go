package rpc

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestGzipCompressor(t *testing.T) {
	gz := &GzipCompressor{}

	testCases := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello world")},
		{"large", []byte(strings.Repeat("test data for compression ", 100))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Compress
			compressed, err := gz.Compress(tc.input)
			if err != nil {
				t.Fatalf("compress failed: %v", err)
			}

			// Decompress
			decompressed, err := gz.Decompress(compressed)
			if err != nil {
				t.Fatalf("decompress failed: %v", err)
			}

			// Compare
			if !bytes.Equal(tc.input, decompressed) {
				t.Errorf("round trip failed: input len=%d, decompressed len=%d",
					len(tc.input), len(decompressed))
			}

			// Check compression actually happened for large data
			if len(tc.input) > 100 && len(compressed) >= len(tc.input) {
				t.Errorf("compression didn't reduce size: input=%d, compressed=%d",
					len(tc.input), len(compressed))
			}
		})
	}
}

func TestShouldCompress(t *testing.T) {
	testCases := []struct {
		size     int
		expected bool
	}{
		{0, false},
		{100, false},
		{1023, false},
		{1024, true},
		{10000, true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("size_%d", tc.size), func(t *testing.T) {
			data := make([]byte, tc.size)
			result := shouldCompress(data)
			if result != tc.expected {
				t.Errorf("shouldCompress(%d bytes) = %v, want %v",
					tc.size, result, tc.expected)
			}
		})
	}
}

func TestCompressorRegistry(t *testing.T) {
	// Test getting registered gzip compressor
	gz, ok := GetCompressor(CompressionGzip)
	if !ok {
		t.Error("gzip compressor not registered")
	}
	if gz.Name() != CompressionGzip {
		t.Errorf("compressor name = %s, want %s", gz.Name(), CompressionGzip)
	}

	// Test getting non-existent compressor
	_, ok = GetCompressor("unknown")
	if ok {
		t.Error("expected false for unknown compressor")
	}
}
