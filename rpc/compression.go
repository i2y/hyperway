package rpc

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

// Compression algorithms
const (
	CompressionIdentity = ""     // No compression
	CompressionGzip     = "gzip" // gzip compression
)

// Compressor interface for compression algorithms
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Name() string
}

// compressorRegistry holds registered compressors
var compressorRegistry = struct {
	sync.RWMutex
	compressors map[string]Compressor
}{
	compressors: make(map[string]Compressor),
}

// RegisterCompressor registers a compressor
func RegisterCompressor(c Compressor) {
	compressorRegistry.Lock()
	defer compressorRegistry.Unlock()
	compressorRegistry.compressors[c.Name()] = c
}

// GetCompressor returns a compressor by name
func GetCompressor(name string) (Compressor, bool) {
	compressorRegistry.RLock()
	defer compressorRegistry.RUnlock()
	c, ok := compressorRegistry.compressors[name]
	return c, ok
}

// GzipCompressor implements gzip compression
type GzipCompressor struct{}

// gzip writer pool to reduce allocations
var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(nil)
	},
}

// gzip reader pool
var gzipReaderPool = sync.Pool{
	New: func() any {
		return new(gzip.Reader)
	},
}

func (g *GzipCompressor) Name() string {
	return CompressionGzip
}

func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Get gzip writer from pool
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(buf)
	defer gzipWriterPool.Put(gz)

	// Write and close
	if _, err := gz.Write(data); err != nil {
		return nil, fmt.Errorf("gzip compress write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip compress close: %w", err)
	}

	// Copy result to avoid buffer reuse issues
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Create reader
	reader := bytes.NewReader(data)

	// Get gzip reader from pool
	gz := gzipReaderPool.Get().(*gzip.Reader)
	defer gzipReaderPool.Put(gz)

	// Reset with new reader
	if err := gz.Reset(reader); err != nil {
		return nil, fmt.Errorf("gzip decompress reset: %w", err)
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Read all data
	if _, err := io.Copy(buf, gz); err != nil {
		return nil, fmt.Errorf("gzip decompress read: %w", err)
	}

	// Copy result
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

// Compression threshold constant
const compressionThreshold = 1024 // 1KB

// shouldCompress determines if a message should be compressed
// based on size threshold (1KB by default)
func shouldCompress(data []byte) bool {
	return len(data) >= compressionThreshold
}

// init registers default compressors
func init() {
	RegisterCompressor(&GzipCompressor{})
}
