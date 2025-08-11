package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"

	reflectutil "github.com/i2y/hyperway/internal/reflect"
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

	// Protocol handler
	streamProtocol StreamProtocol
}

func newClientStreamReader(r *http.Request, ctx *handlerContext, p protocolInfo) *clientStreamReader {
	reader := &clientStreamReader{
		r:              r,
		ctx:            ctx,
		protocol:       p,
		inputType:      ctx.method.InputType,
		streamProtocol: getStreamProtocol(p),
	}

	// Setup decompression
	setupDecompression(reader, r, p)

	// Setup decoding function
	setupDecodingFunction(reader, r, ctx, p)

	return reader
}

func setupDecompression(reader *clientStreamReader, r *http.Request, p protocolInfo) {
	var encoding string
	if p.isGRPC {
		encoding = r.Header.Get("grpc-encoding")
	} else if p.isConnect {
		encoding = r.Header.Get("Connect-Content-Encoding")
	}

	if encoding != "" {
		if compressor, ok := GetCompressor(encoding); ok {
			reader.decompressor = compressor
			reader.hasCompression = true
		}
	}
}

func setupDecodingFunction(reader *clientStreamReader, r *http.Request, ctx *handlerContext, p protocolInfo) {
	// Determine if we're dealing with JSON
	contentType := r.Header.Get("Content-Type")
	isJSON := p.wantsJSON || contentType == contentTypeJSON

	switch {
	case p.isGRPC && !isJSON:
		reader.decodeFunc = createGRPCDecoder(reader.inputType, ctx)
	case ctx.useProtoInput && !isJSON:
		reader.decodeFunc = createProtoDecoder(reader.inputType)
	case isJSON:
		reader.decodeFunc = createJSONDecoder(reader.inputType)
	default:
		reader.decodeFunc = createDefaultDecoder(reader.inputType, ctx)
	}
}

func createGRPCDecoder(inputType reflect.Type, ctx *handlerContext) func([]byte) (any, error) {
	return func(data []byte) (any, error) {
		if ctx.inputCodec != nil {
			// Decode to hyperpb.Message first
			hyperpbMsg, err := ctx.inputCodec.Unmarshal(data)
			if err != nil {
				return nil, err
			}
			defer ctx.inputCodec.ReleaseMessage(hyperpbMsg)

			// Create a new instance of the struct
			result := reflect.New(inputType).Interface()

			// Convert proto to struct using the same utility as unary
			if err := reflectutil.ProtoToStruct(hyperpbMsg.ProtoReflect(), result); err != nil {
				return nil, fmt.Errorf("failed to convert proto to struct: %v", err)
			}

			return result, nil
		}

		// Fallback to direct protobuf decoding
		msg := reflect.New(inputType).Interface()
		if protoMsg, ok := msg.(proto.Message); ok {
			return msg, proto.Unmarshal(data, protoMsg)
		}
		return nil, fmt.Errorf("expected proto.Message, got %T", msg)
	}
}

func createProtoDecoder(inputType reflect.Type) func([]byte) (any, error) {
	return func(data []byte) (any, error) {
		msg := reflect.New(inputType).Interface()
		if protoMsg, ok := msg.(proto.Message); ok {
			return msg, proto.Unmarshal(data, protoMsg)
		}
		return nil, fmt.Errorf("expected proto.Message, got %T", msg)
	}
}

func createJSONDecoder(inputType reflect.Type) func([]byte) (any, error) {
	return func(data []byte) (any, error) {
		msg := reflect.New(inputType).Interface()
		return msg, json.Unmarshal(data, msg)
	}
}

func createDefaultDecoder(inputType reflect.Type, ctx *handlerContext) func([]byte) (any, error) {
	return func(data []byte) (any, error) {
		if ctx.inputCodec != nil {
			// Decode to hyperpb.Message first
			hyperpbMsg, err := ctx.inputCodec.Unmarshal(data)
			if err != nil {
				return nil, err
			}
			defer ctx.inputCodec.ReleaseMessage(hyperpbMsg)

			// Create a new instance of the struct
			result := reflect.New(inputType).Interface()

			// Convert proto to struct using the same utility as unary
			if err := reflectutil.ProtoToStruct(hyperpbMsg.ProtoReflect(), result); err != nil {
				return nil, fmt.Errorf("failed to convert proto to struct: %v", err)
			}

			return result, nil
		}

		// Fallback if no codec
		msg := reflect.New(inputType).Interface()
		if protoMsg, ok := msg.(proto.Message); ok {
			return msg, proto.Unmarshal(data, protoMsg)
		}
		return nil, fmt.Errorf("expected proto.Message, got %T", msg)
	}
}

// Context returns the stream context
func (c *clientStreamReader) Context() context.Context {
	return c.r.Context()
}

// Recv receives a message from the client
func (c *clientStreamReader) Recv() (any, error) {
	// Check initial state
	if err := c.checkInitialState(); err != nil {
		return nil, err
	}

	// Read frame from stream
	data, compressed, isEndOfStream, err := c.streamProtocol.ReadFrame(c.r.Body, c.ctx.options.MaxReceiveMessageSize)

	// Handle end-of-stream
	if isEndOfStream {
		return nil, c.handleEndOfStream(data)
	}

	// Handle read error
	if err != nil {
		return nil, c.handleReadError(err)
	}

	// Decompress if needed
	data, err = c.decompressData(data, compressed)
	if err != nil {
		return nil, err
	}

	// Decode and validate message
	return c.decodeAndValidate(data)
}

// checkInitialState checks if the reader is in a valid state to receive
func (c *clientStreamReader) checkInitialState() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return io.EOF
	}
	if c.err != nil {
		return c.err
	}
	if c.ctx == nil {
		return fmt.Errorf("handler context is nil")
	}
	return nil
}

// handleEndOfStream processes end-of-stream scenarios
func (c *clientStreamReader) handleEndOfStream(data []byte) error {
	// Check if end-of-stream contains an error
	if len(data) > 0 {
		if parsedErr := c.streamProtocol.ParseError(data); parsedErr != nil {
			c.mu.Lock()
			c.err = parsedErr
			c.closed = true
			c.mu.Unlock()
			return parsedErr
		}
	}

	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return io.EOF
}

// handleReadError processes read errors
func (c *clientStreamReader) handleReadError(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.err = err
	if err == io.EOF {
		c.closed = true
	}
	return err
}

// decompressData decompresses the data if needed
func (c *clientStreamReader) decompressData(data []byte, compressed bool) ([]byte, error) {
	if !compressed || !c.hasCompression {
		return data, nil
	}

	decompressed, err := c.decompressor.Decompress(data)
	if err != nil {
		err := NewError(CodeInternal, fmt.Sprintf("failed to decompress message: %v", err))
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		return nil, err
	}

	// Check decompressed size
	if len(decompressed) > c.ctx.options.MaxReceiveMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("decompressed message size %d exceeds maximum allowed size %d",
				len(decompressed), c.ctx.options.MaxReceiveMessageSize))
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		return nil, err
	}

	return decompressed, nil
}

// decodeAndValidate decodes the message and validates it if enabled
func (c *clientStreamReader) decodeAndValidate(data []byte) (any, error) {
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
		if err := c.validateMessage(msg); err != nil {
			return nil, err
		}
	}

	// Update message count
	c.mu.Lock()
	c.messageCount++
	c.mu.Unlock()

	return msg, nil
}

// validateMessage validates the message if it implements Validate
func (c *clientStreamReader) validateMessage(msg any) error {
	validator, ok := msg.(interface{ Validate() error })
	if !ok {
		return nil
	}

	if err := validator.Validate(); err != nil {
		validationErr := NewError(CodeInvalidArgument, fmt.Sprintf("validation failed: %v", err))
		c.mu.Lock()
		c.err = validationErr
		c.mu.Unlock()
		return validationErr
	}

	return nil
}

// Close closes the stream reader
func (c *clientStreamReader) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.r.Body.Close()
}

// typedClientStream implements ClientStream[T]
type typedClientStream[T any] struct {
	reader *clientStreamReader
}

// Recv receives a typed message
func (c *typedClientStream[T]) Recv() (*T, error) {
	msg, err := c.reader.Recv()
	if err != nil {
		return nil, err
	}

	typed, ok := msg.(*T)
	if !ok {
		return nil, fmt.Errorf("unexpected type: got %T, want %T", msg, new(T))
	}

	return typed, nil
}

// Context returns the stream context
func (c *typedClientStream[T]) Context() context.Context {
	return c.reader.Context()
}
