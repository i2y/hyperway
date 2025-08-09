package rpc

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Compression algorithms
const (
	CompressionIdentity = ""     // No compression
	CompressionGzip     = "gzip" // gzip compression
	CompressionBrotli   = "br"   // Brotli compression
	CompressionZstd     = "zstd" // Zstandard compression
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

// selectCompressor selects the best available compressor based on Accept-Encoding header
// Priority: zstd > br > gzip
func selectCompressor(acceptEncoding string) (Compressor, string) {
	// Check in priority order
	compressionPriority := []string{
		CompressionZstd,   // Zstandard has best compression ratio and speed
		CompressionBrotli, // Brotli has good compression ratio
		CompressionGzip,   // gzip is most widely supported
	}

	for _, encoding := range compressionPriority {
		if strings.Contains(acceptEncoding, encoding) {
			if c, ok := GetCompressor(encoding); ok {
				return c, encoding
			}
		}
	}

	return nil, ""
}

// BrotliCompressor implements Brotli compression
type BrotliCompressor struct{}

func (b *BrotliCompressor) Name() string {
	return CompressionBrotli
}

func (b *BrotliCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Create Brotli writer with default compression level
	w := brotli.NewWriterLevel(buf, brotli.DefaultCompression)

	// Write and close
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("brotli compress write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("brotli compress close: %w", err)
	}

	// Copy result to avoid buffer reuse issues
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

func (b *BrotliCompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Create reader
	reader := brotli.NewReader(bytes.NewReader(data))

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Read all data
	if _, err := io.Copy(buf, reader); err != nil {
		return nil, fmt.Errorf("brotli decompress read: %w", err)
	}

	// Copy result
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

// ZstdCompressor implements Zstandard compression
type ZstdCompressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
	mu      sync.Mutex
}

// NewZstdCompressor creates a new Zstandard compressor
func NewZstdCompressor() (*ZstdCompressor, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}

	return &ZstdCompressor{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

func (z *ZstdCompressor) Name() string {
	return CompressionZstd
}

func (z *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	z.mu.Lock()
	defer z.mu.Unlock()

	// Compress data
	compressed := z.encoder.EncodeAll(data, nil)

	return compressed, nil
}

func (z *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	z.mu.Lock()
	defer z.mu.Unlock()

	// Decompress data
	decompressed, err := z.decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}

	return decompressed, nil
}

// init registers default compressors
func init() {
	RegisterCompressor(&GzipCompressor{})
	RegisterCompressor(&BrotliCompressor{})

	// Register Zstd compressor
	if zstdCompressor, err := NewZstdCompressor(); err == nil {
		RegisterCompressor(zstdCompressor)
	}
}
