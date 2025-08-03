package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Additional constants
const (
	colonSplitIndex = 2
)

const (
	// grpcWebMessageFlagData indicates a data frame
	grpcWebMessageFlagData = 0x00
	// grpcWebMessageFlagTrailer indicates a trailer frame
	grpcWebMessageFlagTrailer = 0x80
	// grpcWebFrameHeaderSize is the size of gRPC-Web frame header (1 flag + 4 length)
	grpcWebFrameHeaderSize = 5
)

// grpcWebMode represents the encoding mode for gRPC-Web
type grpcWebMode int

const (
	grpcWebModeBinary grpcWebMode = iota
	grpcWebModeBase64
)

// detectGRPCWebMode detects the gRPC-Web mode from Content-Type header
func detectGRPCWebMode(contentType string) grpcWebMode {
	if strings.Contains(contentType, "application/grpc-web-text") {
		return grpcWebModeBase64
	}
	return grpcWebModeBinary
}

// grpcWebFrame represents a single gRPC-Web frame
type grpcWebFrame struct {
	flag    byte
	payload []byte
}

// isTrailer returns true if this frame contains trailers
func (f *grpcWebFrame) isTrailer() bool {
	return f.flag == grpcWebMessageFlagTrailer
}

// grpcWebFrameWriter writes gRPC-Web frames
type grpcWebFrameWriter struct {
	w              io.Writer
	mode           grpcWebMode
	originalWriter io.Writer // Keep reference to original writer for flushing
}

// newGRPCWebFrameWriter creates a new frame writer
func newGRPCWebFrameWriter(w io.Writer, mode grpcWebMode) *grpcWebFrameWriter {
	if mode == grpcWebModeBase64 {
		// For base64 mode, wrap the writer with base64 encoder
		encoder := base64.NewEncoder(base64.StdEncoding, w)
		return &grpcWebFrameWriter{w: encoder, mode: mode, originalWriter: w}
	}
	return &grpcWebFrameWriter{w: w, mode: mode, originalWriter: w}
}

// writeFrame writes a single frame
func (fw *grpcWebFrameWriter) writeFrame(frame *grpcWebFrame) error {
	// Write frame header
	header := make([]byte, grpcWebFrameHeaderSize)
	header[0] = frame.flag

	// Ensure payload length fits in uint32
	payloadLen := len(frame.payload)
	if payloadLen < 0 || payloadLen > 0x7FFFFFFF { // Max 2GB to be safe
		return fmt.Errorf("payload too large: %d bytes", payloadLen)
	}
	binary.BigEndian.PutUint32(header[1:], uint32(payloadLen))

	if _, err := fw.w.Write(header); err != nil {
		return fmt.Errorf("failed to write frame header: %w", err)
	}

	// Write payload
	if len(frame.payload) > 0 {
		if _, err := fw.w.Write(frame.payload); err != nil {
			return fmt.Errorf("failed to write frame payload: %w", err)
		}
	}

	return nil
}

// writeDataFrame writes a data frame
func (fw *grpcWebFrameWriter) writeDataFrame(data []byte) error {
	return fw.writeFrame(&grpcWebFrame{
		flag:    grpcWebMessageFlagData,
		payload: data,
	})
}

// writeTrailerFrame writes a trailer frame
func (fw *grpcWebFrameWriter) writeTrailerFrame(trailers []byte) error {
	return fw.writeFrame(&grpcWebFrame{
		flag:    grpcWebMessageFlagTrailer,
		payload: trailers,
	})
}

// close closes the writer (important for base64 mode to flush padding)
func (fw *grpcWebFrameWriter) close() error {
	if fw.mode == grpcWebModeBase64 {
		if closer, ok := fw.w.(io.Closer); ok {
			return closer.Close()
		}
	}
	// Always flush the original writer if possible
	if flusher, ok := fw.originalWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// grpcWebFrameReader reads gRPC-Web frames
type grpcWebFrameReader struct {
	r    io.Reader
	mode grpcWebMode
}

// newGRPCWebFrameReader creates a new frame reader
func newGRPCWebFrameReader(r io.Reader, mode grpcWebMode) *grpcWebFrameReader {
	if mode == grpcWebModeBase64 {
		// For base64 mode, wrap the reader with base64 decoder
		decoder := base64.NewDecoder(base64.StdEncoding, r)
		return &grpcWebFrameReader{r: decoder, mode: mode}
	}
	return &grpcWebFrameReader{r: r, mode: mode}
}

// readFrame reads a single frame
func (fr *grpcWebFrameReader) readFrame() (*grpcWebFrame, error) {
	// Read frame header
	header := make([]byte, grpcWebFrameHeaderSize)
	_, err := io.ReadFull(fr.r, header)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read frame header: %w", err)
	}

	flag := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	// Read payload
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(fr.r, payload); err != nil {
			return nil, fmt.Errorf("failed to read frame payload: %w", err)
		}
	}

	return &grpcWebFrame{
		flag:    flag,
		payload: payload,
	}, nil
}

// parseTrailerFrame parses trailer frame payload into HTTP headers
func parseTrailerFrame(payload []byte) http.Header {
	trailers := make(http.Header)

	// Trailers are formatted as HTTP/1.1 headers
	lines := strings.Split(string(payload), "\r\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", colonSplitIndex)
		if len(parts) != colonSplitIndex {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		trailers.Add(key, value)
	}

	return trailers
}

// formatTrailerFrame formats HTTP headers into trailer frame payload
func formatTrailerFrame(trailers http.Header) []byte {
	var buf bytes.Buffer

	// Format trailers as HTTP/1.1 headers
	// gRPC-Web requires lowercase header keys
	for key, values := range trailers {
		for _, value := range values {
			buf.WriteString(strings.ToLower(key))
			buf.WriteString(": ")
			buf.WriteString(value)
			buf.WriteString("\r\n")
		}
	}

	return buf.Bytes()
}

// grpcWebCodec handles encoding/decoding for gRPC-Web
type grpcWebCodec struct {
	mode   grpcWebMode
	isJSON bool
}

// newGRPCWebCodec creates a new gRPC-Web codec
func newGRPCWebCodec(contentType string) *grpcWebCodec {
	return &grpcWebCodec{
		mode:   detectGRPCWebMode(contentType),
		isJSON: strings.Contains(contentType, "+json") || strings.Contains(contentType, "/json"),
	}
}

// isBase64Mode returns true if using base64 encoding
func (c *grpcWebCodec) isBase64Mode() bool {
	return c.mode == grpcWebModeBase64
}

// contentType returns the appropriate Content-Type for responses
func (c *grpcWebCodec) contentType() string {
	suffix := "+proto"
	if c.isJSON {
		suffix = "+json"
	}

	if c.mode == grpcWebModeBase64 {
		return "application/grpc-web-text" + suffix
	}
	return "application/grpc-web" + suffix
}
