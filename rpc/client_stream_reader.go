package rpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"
)

// clientStreamReader implements client-side streaming reader
type clientStreamReader struct {
	r            *http.Request
	ctx          *handlerContext
	protocol     protocolInfo
	mu           sync.Mutex
	err          error
	closed       bool
	messageCount int

	// Cached decoding function
	decodeFunc func([]byte) (any, error)

	// Input type for creating new instances
	inputType reflect.Type

	// Compression settings
	decompressor   Compressor
	hasCompression bool
}

func newClientStreamReader(r *http.Request, ctx *handlerContext, p protocolInfo) *clientStreamReader {
	reader := &clientStreamReader{
		r:         r,
		ctx:       ctx,
		protocol:  p,
		inputType: ctx.method.InputType,
	}

	// Setup decompression
	setupDecompression(reader, r, p)

	// Setup decoding function
	setupDecodingFunction(reader, r, ctx, p)

	return reader
}

func setupDecompression(reader *clientStreamReader, r *http.Request, p protocolInfo) {
	var encoding string
	if p.isGRPC || p.isGRPCWeb {
		encoding = r.Header.Get("grpc-encoding")
	} else if p.isConnect {
		encoding = r.Header.Get("Content-Encoding")
	}

	if encoding != "" && encoding != "identity" {
		if decompressor, ok := GetCompressor(encoding); ok {
			reader.decompressor = decompressor
			reader.hasCompression = true
		}
	}
}

func setupDecodingFunction(reader *clientStreamReader, r *http.Request, ctx *handlerContext, p protocolInfo) {
	contentType := r.Header.Get("Content-Type")
	isJSON := p.wantsJSON || (contentType != "" && (contentType == contentTypeJSON ||
		contentType == contentTypeConnectJSON ||
		contentType == "application/grpc+json" ||
		contentType == "application/grpc-web+json"))

	switch {
	case p.isGRPC && !isJSON:
		// gRPC protobuf decoding
		reader.decodeFunc = func(data []byte) (any, error) {
			protoMsg, err := ctx.inputCodec.Unmarshal(data)
			if err != nil {
				return nil, err
			}
			return protoMsg, nil
		}
	case ctx.useProtoInput && !isJSON:
		// Connect protobuf decoding
		reader.decodeFunc = func(data []byte) (any, error) {
			msg := reflect.New(reader.inputType).Interface()
			if protoMsg, ok := msg.(proto.Message); ok {
				return msg, proto.Unmarshal(data, protoMsg)
			}
			return nil, fmt.Errorf("expected proto.Message, got %T", msg)
		}
	case isJSON:
		// JSON decoding
		reader.decodeFunc = func(data []byte) (any, error) {
			msg := reflect.New(reader.inputType).Interface()
			return msg, json.Unmarshal(data, msg)
		}
	default:
		// Default: use codec
		reader.decodeFunc = func(data []byte) (any, error) {
			protoMsg, err := ctx.inputCodec.Unmarshal(data)
			if err != nil {
				return nil, err
			}
			return protoMsg, nil
		}
	}
}

// Context returns the stream context
func (c *clientStreamReader) Context() context.Context {
	return c.r.Context()
}

// Recv receives a message from the client
func (c *clientStreamReader) Recv() (any, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, io.EOF
	}
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	// Read next message based on protocol
	var data []byte
	var err error

	switch {
	case c.protocol.isGRPC:
		data, err = c.recvGRPCMessage()
	case c.protocol.isConnect:
		data, err = c.recvConnectMessage()
	default:
		// Plain HTTP - read entire body (single message)
		data, err = c.recvPlainMessage()
	}

	if err != nil {
		c.mu.Lock()
		c.err = err
		if err == io.EOF {
			c.closed = true
		}
		c.mu.Unlock()
		return nil, err
	}

	// Check message size
	if len(data) > c.ctx.options.MaxReceiveMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("message size %d exceeds maximum allowed size %d",
				len(data), c.ctx.options.MaxReceiveMessageSize))
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		return nil, err
	}

	// Decode the message
	msg, err := c.decodeFunc(data)
	if err != nil {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		return nil, err
	}

	// Validate if enabled
	if c.ctx.options.EnableValidation {
		if validator, ok := msg.(interface{ Validate() error }); ok {
			if err := validator.Validate(); err != nil {
				validationErr := NewError(CodeInvalidArgument, fmt.Sprintf("validation failed: %v", err))
				c.mu.Lock()
				c.err = validationErr
				c.mu.Unlock()
				return nil, validationErr
			}
		}
	}

	c.mu.Lock()
	c.messageCount++
	c.mu.Unlock()

	return msg, nil
}

// recvGRPCMessage reads a gRPC framed message
func (c *clientStreamReader) recvGRPCMessage() ([]byte, error) {
	// Read frame header
	frameHeader := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(c.r.Body, frameHeader); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to read gRPC frame header: %v", err))
	}

	// Parse frame header
	compressionFlag := frameHeader[0]
	messageLength := binary.BigEndian.Uint32(frameHeader[1:5])

	// Check message size before allocating
	if int(messageLength) > c.ctx.options.MaxReceiveMessageSize {
		return nil, NewError(CodeResourceExhausted,
			fmt.Sprintf("gRPC message size %d exceeds maximum allowed size %d",
				messageLength, c.ctx.options.MaxReceiveMessageSize))
	}

	// Read message body
	data := make([]byte, messageLength)
	if _, err := io.ReadFull(c.r.Body, data); err != nil {
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to read gRPC message body: %v", err))
	}

	// Decompress if needed
	if compressionFlag == 1 && c.hasCompression {
		decompressed, err := c.decompressor.Decompress(data)
		if err != nil {
			return nil, NewError(CodeInternal, fmt.Sprintf("failed to decompress gRPC message: %v", err))
		}
		// Check decompressed size
		if len(decompressed) > c.ctx.options.MaxReceiveMessageSize {
			return nil, NewError(CodeResourceExhausted,
				fmt.Sprintf("decompressed gRPC message size %d exceeds maximum allowed size %d",
					len(decompressed), c.ctx.options.MaxReceiveMessageSize))
		}
		return decompressed, nil
	}

	return data, nil
}

// recvConnectMessage reads a Connect framed message
func (c *clientStreamReader) recvConnectMessage() ([]byte, error) {
	// Read frame header
	frameHeader := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(c.r.Body, frameHeader); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to read Connect frame header: %v", err))
	}

	// Check for end-of-stream marker
	flags := frameHeader[0]
	if flags&0x02 != 0 {
		return c.handleEndOfStream(frameHeader)
	}

	// Parse regular message
	return c.handleRegularMessage(frameHeader)
}

func (c *clientStreamReader) handleEndOfStream(frameHeader []byte) ([]byte, error) {
	messageLength := binary.BigEndian.Uint32(frameHeader[1:5])
	if messageLength == 0 {
		return nil, io.EOF
	}

	// Read the end message (might contain error)
	endData := make([]byte, messageLength)
	if _, err := io.ReadFull(c.r.Body, endData); err != nil {
		return nil, NewError(CodeInternal, "failed to read Connect end message")
	}

	// Check if it contains an error
	var endMsg map[string]any
	if err := json.Unmarshal(endData, &endMsg); err == nil {
		if errData, ok := endMsg["error"].(map[string]any); ok {
			code := CodeInternal
			if codeStr, ok := errData["code"].(string); ok {
				code = Code(codeStr)
			}
			message := "stream error"
			if msgStr, ok := errData["message"].(string); ok {
				message = msgStr
			}
			return nil, NewError(code, message)
		}
	}
	return nil, io.EOF
}

func (c *clientStreamReader) handleRegularMessage(frameHeader []byte) ([]byte, error) {
	const compressionFlagMask = 0x01
	compressionFlag := frameHeader[0]&compressionFlagMask != 0
	messageLength := binary.BigEndian.Uint32(frameHeader[1:5])

	// Check message size before allocating
	if int(messageLength) > c.ctx.options.MaxReceiveMessageSize {
		return nil, NewError(CodeResourceExhausted,
			fmt.Sprintf("Connect message size %d exceeds maximum allowed size %d",
				messageLength, c.ctx.options.MaxReceiveMessageSize))
	}

	// Read message body
	data := make([]byte, messageLength)
	if _, err := io.ReadFull(c.r.Body, data); err != nil {
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to read Connect message body: %v", err))
	}

	// Decompress if needed
	if compressionFlag && c.hasCompression {
		return c.decompressMessage(data)
	}

	return data, nil
}

func (c *clientStreamReader) decompressMessage(data []byte) ([]byte, error) {
	decompressed, err := c.decompressor.Decompress(data)
	if err != nil {
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to decompress Connect message: %v", err))
	}
	// Check decompressed size
	if len(decompressed) > c.ctx.options.MaxReceiveMessageSize {
		return nil, NewError(CodeResourceExhausted,
			fmt.Sprintf("decompressed Connect message size %d exceeds maximum allowed size %d",
				len(decompressed), c.ctx.options.MaxReceiveMessageSize))
	}
	return decompressed, nil
}

// recvPlainMessage reads a plain HTTP message (single message only)
func (c *clientStreamReader) recvPlainMessage() ([]byte, error) {
	// For plain HTTP, we only support single message (unary-like)
	// True streaming requires WebSocket or SSE
	if c.messageCount > 0 {
		return nil, io.EOF
	}

	// Read entire body
	limitedReader := io.LimitReader(c.r.Body, int64(c.ctx.options.MaxReceiveMessageSize)+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, NewError(CodeInternal, fmt.Sprintf("failed to read request body: %v", err))
	}

	// Check if we exceeded the size limit
	if len(data) > c.ctx.options.MaxReceiveMessageSize {
		return nil, NewError(CodeResourceExhausted,
			fmt.Sprintf("message size %d exceeds maximum allowed size %d",
				len(data), c.ctx.options.MaxReceiveMessageSize))
	}

	// Decompress if needed
	if c.hasCompression {
		decompressed, err := c.decompressor.Decompress(data)
		if err != nil {
			return nil, NewError(CodeInternal, fmt.Sprintf("failed to decompress message: %v", err))
		}
		return decompressed, nil
	}

	return data, nil
}

// Close closes the stream reader
func (c *clientStreamReader) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	// Drain any remaining data to allow connection reuse
	_, _ = io.Copy(io.Discard, c.r.Body)
	return c.r.Body.Close()
}

// Implement typed client stream
type typedClientStream[T any] struct {
	reader *clientStreamReader
}

func (c *typedClientStream[T]) Recv() (*T, error) {
	msg, err := c.reader.Recv()
	if err != nil {
		return nil, err
	}
	// Type assertion should be safe since we control the decode function
	return msg.(*T), nil
}

func (c *typedClientStream[T]) Context() context.Context {
	return c.reader.Context()
}
