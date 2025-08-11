package rpc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// StreamProtocol defines the interface for protocol-specific streaming operations
type StreamProtocol interface {
	// ReadFrame reads a single frame from the stream
	// Returns: data, compressed, isEndOfStream, error
	ReadFrame(r io.Reader, maxMessageSize int) ([]byte, bool, bool, error)

	// WriteFrame writes a single frame to the stream
	WriteFrame(w io.Writer, data []byte, compressed bool) error

	// WriteEndOfStream writes an end-of-stream marker (if applicable)
	WriteEndOfStream(w io.Writer) error

	// SetStreamHeaders sets protocol-specific headers for streaming
	SetStreamHeaders(h http.Header, contentType, compressionType string)

	// SetResponseHeaders sets protocol-specific headers for unary responses
	SetResponseHeaders(h http.Header, compressed bool, compressionType string)

	// FormatError formats an error for the protocol
	FormatError(err *Error) []byte

	// ParseError parses an error from protocol-specific format
	ParseError(data []byte) *Error

	// SupportsTrailers returns true if the protocol uses HTTP trailers for errors
	SupportsTrailers() bool
}

// grpcStreamProtocol implements StreamProtocol for gRPC
type grpcStreamProtocol struct{}

func (g *grpcStreamProtocol) ReadFrame(r io.Reader, maxMessageSize int) (data []byte, compressed, isEndOfStream bool, err error) {
	// Read frame header
	frameHeader := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(r, frameHeader); err != nil {
		if err == io.EOF {
			return nil, false, false, io.EOF
		}
		return nil, false, false, NewError(CodeInternal, fmt.Sprintf("failed to read gRPC frame header: %v", err))
	}

	// Parse frame header
	compressionFlag := frameHeader[0]
	messageLength := binary.BigEndian.Uint32(frameHeader[1:5])

	// Check message size
	if int(messageLength) > maxMessageSize {
		return nil, false, false, NewError(CodeResourceExhausted,
			fmt.Sprintf("gRPC message size %d exceeds maximum allowed size %d",
				messageLength, maxMessageSize))
	}

	// Read message body
	data = make([]byte, messageLength)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, false, false, NewError(CodeInternal, fmt.Sprintf("failed to read gRPC message body: %v", err))
	}

	// Return data with compression flag
	compressed = compressionFlag == 1
	return data, compressed, false, nil
}

func (g *grpcStreamProtocol) WriteFrame(w io.Writer, data []byte, compressed bool) error {
	// gRPC frame format: 1 byte flags + 4 bytes length + data
	frame := make([]byte, frameHeaderLength+len(data))

	// Set compression flag
	if compressed {
		frame[0] = 1
	} else {
		frame[0] = 0
	}

	// Set length (big-endian)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits

	// Copy data
	copy(frame[5:], data)

	// Write frame
	_, err := w.Write(frame)
	return err
}

func (g *grpcStreamProtocol) WriteEndOfStream(w io.Writer) error {
	// gRPC doesn't have an explicit end-of-stream marker
	// Stream ends when the response is complete
	return nil
}

func (g *grpcStreamProtocol) SetStreamHeaders(h http.Header, contentType, compressionType string) {
	h.Set("Content-Type", contentType)
	h.Set("grpc-accept-encoding", "gzip, br, zstd")
	if compressionType != "" {
		h.Set("grpc-encoding", compressionType)
	}
}

func (g *grpcStreamProtocol) SetResponseHeaders(h http.Header, compressed bool, compressionType string) {
	h.Set("grpc-accept-encoding", "gzip, br, zstd")
	if compressed && compressionType != "" {
		h.Set("grpc-encoding", compressionType)
	}
}

func (g *grpcStreamProtocol) FormatError(err *Error) []byte {
	// gRPC errors are sent in trailers, not as frames
	// This returns empty since errors are handled via trailers
	return nil
}

func (g *grpcStreamProtocol) ParseError(data []byte) *Error {
	// gRPC doesn't send errors as frames
	return nil
}

func (g *grpcStreamProtocol) SupportsTrailers() bool {
	return true
}

// connectStreamProtocol implements StreamProtocol for Connect
type connectStreamProtocol struct{}

func (c *connectStreamProtocol) ReadFrame(r io.Reader, maxMessageSize int) (data []byte, compressed, isEndOfStream bool, err error) {
	// Read frame header
	frameHeader := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(r, frameHeader); err != nil {
		if err == io.EOF {
			return nil, false, false, io.EOF
		}
		return nil, false, false, NewError(CodeInternal, fmt.Sprintf("failed to read Connect frame header: %v", err))
	}

	// Check for end-of-stream marker and compression
	flags := frameHeader[0]
	messageLength := binary.BigEndian.Uint32(frameHeader[1:5])

	// Check message size
	if int(messageLength) > maxMessageSize {
		return nil, false, false, NewError(CodeResourceExhausted,
			fmt.Sprintf("Connect message size %d exceeds maximum allowed size %d",
				messageLength, maxMessageSize))
	}

	// Read message body
	data = make([]byte, messageLength)
	if messageLength > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, false, false, NewError(CodeInternal, "failed to read Connect message body")
		}
	}

	// Check flags
	const (
		compressionFlag = 0x01
		endOfStreamFlag = 0x02
	)
	compressed = (flags & compressionFlag) != 0
	isEndOfStream = (flags & endOfStreamFlag) != 0

	return data, compressed, isEndOfStream, nil
}

func (c *connectStreamProtocol) WriteFrame(w io.Writer, data []byte, compressed bool) error {
	// Connect frame format: 1 byte flags + 4 bytes length + data
	frame := make([]byte, frameHeaderLength+len(data))

	// Set compression flag
	if compressed {
		frame[0] = 1
	} else {
		frame[0] = 0
	}

	// Set length (big-endian)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits

	// Copy data
	copy(frame[5:], data)

	// Write frame
	_, err := w.Write(frame)
	return err
}

func (c *connectStreamProtocol) WriteEndOfStream(w io.Writer) error {
	// Connect end-of-stream frame with empty JSON object
	endPayload := []byte("{}")
	frame := make([]byte, frameHeaderLength+len(endPayload))

	// Set end-of-stream flag
	frame[0] = 0x02

	// Set length
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(endPayload))) //nolint:gosec // length is bounded

	// Copy payload
	copy(frame[5:], endPayload)

	// Write frame
	_, err := w.Write(frame)
	return err
}

func (c *connectStreamProtocol) SetStreamHeaders(h http.Header, contentType, compressionType string) {
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "no-cache")
	if compressionType != "" {
		h.Set("Connect-Content-Encoding", compressionType)
	}
}

func (c *connectStreamProtocol) SetResponseHeaders(h http.Header, compressed bool, compressionType string) {
	if compressed && compressionType != "" {
		h.Set("Connect-Content-Encoding", compressionType)
	}
}

func (c *connectStreamProtocol) FormatError(err *Error) []byte {
	// Connect sends errors as JSON in end-of-stream frames
	errData := map[string]any{
		"error": map[string]any{
			"code":    string(err.Code),
			"message": err.Message,
		},
	}
	if err.Details != nil {
		errData["error"].(map[string]any)["details"] = err.Details
	}

	data, _ := json.Marshal(errData)
	return data
}

func (c *connectStreamProtocol) ParseError(data []byte) *Error {
	// Parse Connect error from JSON
	var errData struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details,omitempty"`
		} `json:"error"`
	}

	if err := json.Unmarshal(data, &errData); err != nil {
		return nil
	}

	if errData.Error.Code != "" {
		return &Error{
			Code:    Code(errData.Error.Code),
			Message: errData.Error.Message,
			Details: errData.Error.Details,
		}
	}

	return nil
}

func (c *connectStreamProtocol) SupportsTrailers() bool {
	return false
}

// getStreamProtocol returns the appropriate StreamProtocol implementation
func getStreamProtocol(p protocolInfo) StreamProtocol {
	if p.isGRPC {
		return &grpcStreamProtocol{}
	}
	if p.isConnect {
		return &connectStreamProtocol{}
	}
	// Default to Connect protocol for plain HTTP
	return &connectStreamProtocol{}
}
