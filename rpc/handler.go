package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/i2y/hyperway/codec"
	reflectutil "github.com/i2y/hyperway/internal/reflect"
	"github.com/i2y/hyperway/schema"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

// Context keys.
const (
	contextKeyCancel contextKey = "cancel"
)

// Frame header constants
const (
	frameHeaderSize     = 5
	frameFlagCompressed = 1

	// Buffer pool sizes
	defaultBufferSize = 4096
	maxBufferSize     = 1024 * 1024 // 1MB

	// gRPC status codes
	grpcStatusOK                 = 0
	grpcStatusCanceled           = 1
	grpcStatusUnknown            = 2
	grpcStatusInvalidArgument    = 3
	grpcStatusDeadlineExceeded   = 4
	grpcStatusNotFound           = 5
	grpcStatusAlreadyExists      = 6
	grpcStatusPermissionDenied   = 7
	grpcStatusResourceExhausted  = 8
	grpcStatusFailedPrecondition = 9
	grpcStatusAborted            = 10
	grpcStatusOutOfRange         = 11
	grpcStatusUnimplemented      = 12
	grpcStatusInternal           = 13
	grpcStatusUnavailable        = 14
	grpcStatusDataLoss           = 15
	grpcStatusUnauthenticated    = 16

	// Content type constants
	contentTypeProto        = "application/proto"
	contentTypeConnectProto = "application/connect+proto"
)

// grpcStatusCodeMap maps error codes to gRPC status codes.
var grpcStatusCodeMap = map[Code]int{
	CodeCanceled:           grpcStatusCanceled,
	CodeUnknown:            grpcStatusUnknown,
	CodeInvalidArgument:    grpcStatusInvalidArgument,
	CodeDeadlineExceeded:   grpcStatusDeadlineExceeded,
	CodeNotFound:           grpcStatusNotFound,
	CodeAlreadyExists:      grpcStatusAlreadyExists,
	CodePermissionDenied:   grpcStatusPermissionDenied,
	CodeResourceExhausted:  grpcStatusResourceExhausted,
	CodeFailedPrecondition: grpcStatusFailedPrecondition,
	CodeAborted:            grpcStatusAborted,
	CodeOutOfRange:         grpcStatusOutOfRange,
	CodeUnimplemented:      grpcStatusUnimplemented,
	CodeInternal:           grpcStatusInternal,
	CodeUnavailable:        grpcStatusUnavailable,
	CodeDataLoss:           grpcStatusDataLoss,
	CodeUnauthenticated:    grpcStatusUnauthenticated,
}

// grpcStatusCode returns the gRPC status code for an error code.
func grpcStatusCode(code Code) int {
	if status, ok := grpcStatusCodeMap[code]; ok {
		return status
	}
	return grpcStatusUnknown
}

// Buffer pools for reducing allocations
var (
	// Pool for frame headers (5 bytes)
	frameHeaderPool = sync.Pool{
		New: func() any {
			b := make([]byte, frameHeaderSize)
			return &b
		},
	}

	// Pool for general purpose buffers
	bufferPool = sync.Pool{
		New: func() any {
			return &bytes.Buffer{}
		},
	}

	// Pool for byte slices
	byteSlicePool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, defaultBufferSize)
			return &b
		},
	}

	// Pool for handler contexts
	handlerContextPool = sync.Pool{
		New: func() any {
			return &handlerContext{}
		},
	}
)

// handlerContext holds the context for a handler.
type handlerContext struct {
	inputCodec   *codec.Codec
	outputCodec  *codec.Codec
	method       *Method
	validator    interface{ Struct(any) error }
	options      ServiceOptions
	interceptors []Interceptor
	handlerInfo  *HandlerInfo // Cached handler metadata
}

// createHTTPHandler creates an HTTP handler for a method.
func (s *Service) createHTTPHandler(method *Method) http.HandlerFunc {
	ctx, err := s.prepareHandlerContext(method)
	if err != nil {
		return errorHandler(err)
	}

	// Create a handler that supports Connect protocol
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleRequest(w, r, ctx)
	})

	// Wrap with Connect protocol support
	// The handler already supports JSON, and Vanguard will handle protocol translation
	return handler
}

// prepareHandlerContext prepares the handler context.
func (s *Service) prepareHandlerContext(method *Method) (*handlerContext, error) {
	// Get cached handler info
	handlerInfo, err := GetHandlerInfo(method.Handler)
	if err != nil {
		return nil, err
	}

	// Build message descriptors (cached in builder)
	inputDesc, err := s.builder.BuildMessage(handlerInfo.InputType)
	if err != nil {
		return nil, fmt.Errorf("failed to build input descriptor: %w", err)
	}

	outputDesc, err := s.builder.BuildMessage(handlerInfo.OutputType)
	if err != nil {
		return nil, fmt.Errorf("failed to build output descriptor: %w", err)
	}

	// Create codecs
	inputCodec, err := codec.New(inputDesc, codec.DefaultOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to create input codec: %w", err)
	}

	outputCodec, err := codec.New(outputDesc, codec.DefaultOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to create output codec: %w", err)
	}

	// Get context from pool
	ctx := handlerContextPool.Get().(*handlerContext)

	// Reset and populate
	ctx.inputCodec = inputCodec
	ctx.outputCodec = outputCodec
	ctx.method = method
	ctx.validator = s.validator
	ctx.options = s.options
	ctx.handlerInfo = handlerInfo

	// Clear and rebuild interceptors slice
	ctx.interceptors = ctx.interceptors[:0]
	// Add method-specific interceptors
	ctx.interceptors = append(ctx.interceptors, method.Options.Interceptors...)
	// Add service-level interceptors
	ctx.interceptors = append(ctx.interceptors, s.options.Interceptors...)

	return ctx, nil
}

// protocolInfo contains information about the request protocol.
type protocolInfo struct {
	isConnect bool
	isGRPC    bool
}

// detectProtocol detects the protocol type from the request.
func detectProtocol(r *http.Request) protocolInfo {
	contentType := r.Header.Get("Content-Type")
	connectProtocol := r.Header.Get("Connect-Protocol-Version")
	return protocolInfo{
		isConnect: connectProtocol == "1",
		isGRPC:    strings.HasPrefix(contentType, "application/grpc"),
	}
}

// handleMethodNotAllowed handles non-POST requests.
func (s *Service) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request, p protocolInfo) {
	if p.isConnect {
		s.writeConnectError(w, r, NewError(CodeUnimplemented, "Method not allowed"))
	} else if p.isGRPC {
		w.Header().Set("grpc-status", fmt.Sprintf("%d", grpcStatusUnimplemented))
		w.Header().Set("grpc-message", "Method not allowed")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// parseRequestTimeout parses timeout headers and returns a context with timeout if applicable.
func parseRequestTimeout(r *http.Request, isConnect bool) context.Context {
	ctx := r.Context()

	if isConnect {
		if timeoutMs := r.Header.Get("Connect-Timeout-Ms"); timeoutMs != "" {
			if ms, err := strconv.ParseInt(timeoutMs, 10, 64); err == nil && ms > 0 {
				timeout := time.Duration(ms) * time.Millisecond
				newCtx, cancel := context.WithTimeout(ctx, timeout)
				// Store cancel func in context for deferred cleanup
				return context.WithValue(newCtx, contextKeyCancel, cancel)
			}
		}
	}

	return ctx
}

// handleRequest handles an HTTP request.
func (s *Service) handleRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext) {
	// Detect protocol
	proto := detectProtocol(r)

	// Only accept POST
	if r.Method != http.MethodPost {
		s.handleMethodNotAllowed(w, r, proto)
		return
	}

	// Parse timeout
	reqCtx := parseRequestTimeout(r, proto.isConnect)
	if cancel, ok := reqCtx.Value(contextKeyCancel).(context.CancelFunc); ok {
		defer cancel()
		// Remove cancel from context to avoid leaking it
		reqCtx = context.WithValue(reqCtx, contextKeyCancel, nil)
	}

	// gRPC requires special handling
	if proto.isGRPC {
		s.handleGRPCRequest(w, r, ctx)
		return
	}

	// Read body using pooled buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	_, err := io.Copy(buf, r.Body)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("failed to read body: %w", err))
		return
	}
	body := buf.Bytes()
	defer func() { _ = r.Body.Close() }()

	// Handle Connect compression (Content-Encoding header)
	if encoding := r.Header.Get("Content-Encoding"); encoding == CompressionGzip {
		compressor, ok := GetCompressor(CompressionGzip)
		if !ok {
			s.writeError(w, r, fmt.Errorf("gzip decompression not available"))
			return
		}

		decompressed, err := compressor.Decompress(body)
		if err != nil {
			s.writeError(w, r, fmt.Errorf("failed to decompress request: %w", err))
			return
		}
		body = decompressed
	}

	// Decode input
	inputVal, err := s.decodeInput(r.Header.Get("Content-Type"), body, ctx)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	// Validate if enabled
	if err := s.validateInput(inputVal, ctx); err != nil {
		s.writeError(w, r, err)
		return
	}

	// Call handler with potentially timeout-limited context
	output, err := s.callHandler(reqCtx, inputVal, ctx)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	// Encode and send response
	if err := s.encodeResponse(w, r, output, ctx, proto.isConnect); err != nil {
		s.writeError(w, r, err)
	}
}

// errorHandler returns a handler that always returns an error.
func errorHandler(err error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a dummy service to use writeError
		s := &Service{}
		s.writeError(w, r, err)
	}
}

// writeError writes an error response.
func (s *Service) writeError(w http.ResponseWriter, r *http.Request, err error) {
	// Check if this is a Connect protocol request
	connectProtocol := r.Header.Get("Connect-Protocol-Version")
	isConnect := connectProtocol == "1"

	// Convert error to our Error type if needed
	var rpcErr *Error
	if e, ok := err.(*Error); ok {
		rpcErr = e
	} else {
		// Map specific error types to appropriate codes
		switch {
		case err == context.DeadlineExceeded:
			rpcErr = NewError(CodeDeadlineExceeded, "Request deadline exceeded")
		case err == context.Canceled:
			rpcErr = NewError(CodeCanceled, "Request was canceled")
		case strings.Contains(err.Error(), "validation failed"):
			rpcErr = NewError(CodeInvalidArgument, err.Error())
		default:
			rpcErr = NewError(CodeInternal, err.Error())
		}
	}

	if isConnect {
		s.writeConnectError(w, r, rpcErr)
	} else {
		// Standard HTTP error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rpcErr.Code.HTTPStatusCode())
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": rpcErr.Error(),
		})
	}
}

// writeConnectError writes a Connect protocol error response.
func (s *Service) writeConnectError(w http.ResponseWriter, r *http.Request, err *Error) {
	// Determine response content type based on request
	contentType := r.Header.Get("Content-Type")
	isProto := contentType == contentTypeProto || contentType == contentTypeConnectProto

	if isProto {
		w.Header().Set("Content-Type", contentTypeProto)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusOK) // Connect uses 200 with error in body

	response := map[string]any{
		"code":    string(err.Code),
		"message": err.Message,
	}
	if err.Details != nil {
		response["details"] = err.Details
	}

	// For now, always encode as JSON even for proto requests
	// TODO: Implement proper proto error encoding
	_ = json.NewEncoder(w).Encode(response)
}

// HandlerFunc is the signature for RPC handlers.
type HandlerFunc func(context.Context, any) (any, error)

// decodeInput decodes the input based on content type.
func (s *Service) decodeInput(contentType string, body []byte, ctx *handlerContext) (reflect.Value, error) {
	// Create input instance
	inputVal := reflect.New(ctx.method.InputType)

	// Decode based on content type
	switch contentType {
	case "application/json", "application/connect+json":
		if err := json.Unmarshal(body, inputVal.Interface()); err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal JSON: %v", err)
		}
	case "application/protobuf", "application/x-protobuf", contentTypeProto, contentTypeConnectProto:
		// Decode protobuf
		msg, err := ctx.inputCodec.Unmarshal(body)
		if err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal protobuf: %v", err)
		}
		defer ctx.inputCodec.ReleaseMessage(msg)

		// Convert to struct
		if err := reflectutil.ProtoToStruct(msg.ProtoReflect(), inputVal.Interface()); err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to convert proto to struct: %v", err)
		}
	default:
		// Default to JSON
		if err := json.Unmarshal(body, inputVal.Interface()); err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal: %v", err)
		}
	}

	return inputVal, nil
}

// validateInput validates the input if enabled.
func (s *Service) validateInput(inputVal reflect.Value, ctx *handlerContext) error {
	shouldValidate := ctx.options.EnableValidation
	if ctx.method.Options.Validate != nil {
		shouldValidate = *ctx.method.Options.Validate
	}
	if shouldValidate {
		// Standard validation
		if err := ctx.validator.Struct(inputVal.Elem().Interface()); err != nil {
			return NewErrorf(CodeInvalidArgument, "validation failed: %v", err)
		}

		// Oneof validation
		if err := schema.ValidateOneof(inputVal.Elem().Type(), inputVal.Elem().Interface()); err != nil {
			return fmt.Errorf("oneof validation failed: %w", err)
		}
	}
	return nil
}

// callHandler calls the handler function.
func (s *Service) callHandler(ctx context.Context, inputVal reflect.Value, hctx *handlerContext) (any, error) {
	// Create the base handler function using cached handler info
	baseHandler := func(ctx context.Context, req any) (any, error) {
		// Use cached handler value for better performance
		results := hctx.handlerInfo.HandlerValue.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(req),
		})

		// Check error
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}

		return results[0].Interface(), nil
	}

	// Apply interceptors if any
	if len(hctx.interceptors) > 0 {
		// Build the handler chain
		handler := baseHandler

		// Apply interceptors in reverse order
		for i := len(hctx.interceptors) - 1; i >= 0; i-- {
			interceptor := hctx.interceptors[i]
			next := handler
			handler = func(ctx context.Context, req any) (any, error) {
				return interceptor.Intercept(ctx, hctx.method.Name, req, next)
			}
		}

		// Call with interceptors
		return handler(ctx, inputVal.Interface())
	}

	// Call without interceptors
	return baseHandler(ctx, inputVal.Interface())
}

// encodeResponse encodes and sends the response.
func (s *Service) encodeResponse(w http.ResponseWriter, r *http.Request, output any, ctx *handlerContext, _ bool) error {
	// Determine content type
	contentType := determineContentType(r)

	// Check if client accepts compression
	canCompress := strings.Contains(r.Header.Get("Accept-Encoding"), CompressionGzip)

	// DEBUG: Log content type detection
	// fmt.Printf("DEBUG: Request Content-Type: %s, Accept: %s, Determined: %s\n",
	//     r.Header.Get("Content-Type"), r.Header.Get("Accept"), contentType)

	// Handle different content types
	if isProtobufContentType(contentType) {
		return s.encodeProtobufResponse(w, output, ctx, canCompress)
	}

	// Default to JSON
	return s.encodeJSONResponse(w, output, canCompress)
}

// determineContentType determines the response content type
func determineContentType(r *http.Request) string {
	contentType := r.Header.Get("Content-Type")
	accept := r.Header.Get("Accept")

	// If Accept header is specified and different from Content-Type, prefer Accept
	if accept != "" && accept != "*/*" {
		return accept
	}
	return contentType
}

// isProtobufContentType checks if the content type is protobuf
func isProtobufContentType(contentType string) bool {
	return contentType == "application/protobuf" ||
		contentType == "application/x-protobuf" ||
		contentType == contentTypeProto ||
		contentType == contentTypeConnectProto
}

// encodeProtobufResponse encodes a protobuf response
func (s *Service) encodeProtobufResponse(w http.ResponseWriter, output any, ctx *handlerContext, canCompress bool) error {
	// Convert to JSON first
	jsonData, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Use hyperpb codec to create and unmarshal message
	msg, err := ctx.outputCodec.UnmarshalFromJSON(jsonData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON to proto: %w", err)
	}
	defer ctx.outputCodec.ReleaseMessage(msg)

	// Marshal to protobuf binary using hyperpb codec
	data, err := ctx.outputCodec.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Apply compression if needed
	data = s.maybeCompress(data, w, canCompress)

	w.Header().Set("Content-Type", contentTypeProto)
	_, _ = w.Write(data)
	return nil
}

// encodeJSONResponse encodes a JSON response
func (s *Service) encodeJSONResponse(w http.ResponseWriter, output any, canCompress bool) error {
	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Apply compression if needed
	data = s.maybeCompress(data, w, canCompress)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
	return nil
}

// maybeCompress compresses data if conditions are met
func (s *Service) maybeCompress(data []byte, w http.ResponseWriter, canCompress bool) []byte {
	if !canCompress || !shouldCompress(data) {
		return data
	}

	compressor, ok := GetCompressor(CompressionGzip)
	if !ok {
		return data
	}

	compressedData, err := compressor.Compress(data)
	if err != nil || len(compressedData) >= len(data) {
		return data
	}

	w.Header().Set("Content-Encoding", CompressionGzip)
	return compressedData
}

// handleGRPCRequest handles a gRPC protocol request.
func (s *Service) handleGRPCRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext) {
	// gRPC uses a 5-byte message framing
	// Get frame header from pool
	frameHeaderPtr := frameHeaderPool.Get().(*[]byte)
	frameHeader := *frameHeaderPtr
	defer frameHeaderPool.Put(frameHeaderPtr)

	if _, err := io.ReadFull(r.Body, frameHeader); err != nil {
		s.writeGRPCError(w, NewError(CodeInternal, "failed to read frame header"))
		return
	}

	// Parse frame header
	compressed := frameHeader[0] == frameFlagCompressed
	// Extract 32-bit message length from bytes 1-4 (big-endian)
	const (
		shift24 = 24
		shift16 = 16
		shift8  = 8
	)
	messageLength := int(frameHeader[1])<<shift24 | int(frameHeader[2])<<shift16 | int(frameHeader[3])<<shift8 | int(frameHeader[4])

	// Get appropriately sized buffer from pool
	var message []byte
	if messageLength <= maxBufferSize {
		msgPtr := byteSlicePool.Get().(*[]byte)
		if cap(*msgPtr) < messageLength {
			*msgPtr = make([]byte, messageLength)
		} else {
			*msgPtr = (*msgPtr)[:messageLength]
		}
		message = *msgPtr
		defer func() {
			*msgPtr = message[:0] // Reset slice
			byteSlicePool.Put(msgPtr)
		}()
	} else {
		// For very large messages, allocate directly
		message = make([]byte, messageLength)
	}

	if _, err := io.ReadFull(r.Body, message); err != nil {
		s.writeGRPCError(w, NewError(CodeInternal, "failed to read message"))
		return
	}

	// Decompress if needed
	if compressed {
		// gRPC uses gzip by default
		compressor, ok := GetCompressor(CompressionGzip)
		if !ok {
			s.writeGRPCError(w, NewError(CodeUnimplemented, "gzip compression not available"))
			return
		}

		decompressed, err := compressor.Decompress(message)
		if err != nil {
			s.writeGRPCError(w, NewErrorf(CodeInternal, "decompression failed: %v", err))
			return
		}
		message = decompressed
	}

	// Decode input
	inputVal, err := s.decodeGRPCInput(message, ctx)
	if err != nil {
		s.writeGRPCError(w, err)
		return
	}

	// Validate if enabled
	if err := s.validateInput(inputVal, ctx); err != nil {
		s.writeGRPCError(w, err)
		return
	}

	// Call handler with potentially timeout-limited context (gRPC deadline)
	reqCtx := r.Context()
	if deadline := r.Header.Get("grpc-timeout"); deadline != "" {
		// Parse gRPC timeout format (e.g., "10S" for 10 seconds)
		if timeout, err := parseGRPCTimeout(deadline); err == nil && timeout > 0 {
			var cancel context.CancelFunc
			reqCtx, cancel = context.WithTimeout(reqCtx, timeout)
			defer cancel()
		}
	}

	// Call handler
	output, err := s.callHandler(reqCtx, inputVal, ctx)
	if err != nil {
		s.writeGRPCError(w, err)
		return
	}

	// Encode and send response
	if err := s.encodeGRPCResponse(w, r, output, ctx); err != nil {
		s.writeGRPCError(w, err)
	}
}

// decodeGRPCInput decodes gRPC protobuf input.
func (s *Service) decodeGRPCInput(data []byte, ctx *handlerContext) (reflect.Value, error) {
	// Create input instance
	inputVal := reflect.New(ctx.method.InputType)

	// Decode protobuf
	msg, err := ctx.inputCodec.Unmarshal(data)
	if err != nil {
		return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal protobuf: %v", err)
	}
	defer ctx.inputCodec.ReleaseMessage(msg)

	// Convert to struct
	if err := reflectutil.ProtoToStruct(msg.ProtoReflect(), inputVal.Interface()); err != nil {
		return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to convert proto to struct: %v", err)
	}

	return inputVal, nil
}

// encodeGRPCResponse encodes and sends a gRPC response.
func (s *Service) encodeGRPCResponse(w http.ResponseWriter, r *http.Request, output any, ctx *handlerContext) error {
	// Set gRPC headers
	w.Header().Set("Content-Type", "application/grpc+proto")
	// Declare trailers that will be sent
	w.Header().Set("Trailer", "grpc-status, grpc-message")
	w.WriteHeader(http.StatusOK)

	// Convert to protobuf
	jsonData, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Use hyperpb codec to create and unmarshal message
	msg, err := ctx.outputCodec.UnmarshalFromJSON(jsonData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON to proto: %w", err)
	}
	defer ctx.outputCodec.ReleaseMessage(msg)

	// Marshal to protobuf binary using hyperpb codec
	data, err := ctx.outputCodec.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Check if compression should be used
	compressed := false
	encodingHeader := r.Header.Get("grpc-encoding")
	if encodingHeader == CompressionGzip && shouldCompress(data) {
		compressor, ok := GetCompressor(CompressionGzip)
		if ok {
			compressedData, err := compressor.Compress(data)
			if err == nil && len(compressedData) < len(data) {
				data = compressedData
				compressed = true
				w.Header().Set("grpc-encoding", CompressionGzip)
			}
		}
	}

	// Write gRPC frame using pooled buffer
	framePtr := frameHeaderPool.Get().(*[]byte)
	frame := *framePtr
	defer frameHeaderPool.Put(framePtr)

	if compressed {
		frame[0] = frameFlagCompressed
	} else {
		frame[0] = 0
	}
	const (
		shift24 = 24
		shift16 = 16
		shift8  = 8
	)
	frame[1] = byte(len(data) >> shift24)
	frame[2] = byte(len(data) >> shift16)
	frame[3] = byte(len(data) >> shift8)
	frame[4] = byte(len(data))

	_, _ = w.Write(frame)
	_, _ = w.Write(data)

	// Send trailers after writing the body
	// In HTTP/2, trailers are sent as a separate HEADERS frame with END_STREAM flag
	// The Go HTTP/2 server automatically sends trailers when we set them after writing the body
	trailer := w.Header()
	trailer.Set("grpc-status", "0")
	trailer.Set("grpc-message", "")

	// Flush to ensure trailers are sent
	// This is critical for HTTP/2 trailers to be properly sent
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	return nil
}

// writeGRPCError writes a gRPC error response.
func (s *Service) writeGRPCError(w http.ResponseWriter, err error) {
	// Convert to our Error type if needed
	var rpcErr *Error
	if e, ok := err.(*Error); ok {
		rpcErr = e
	} else {
		rpcErr = NewError(CodeInternal, err.Error())
	}

	w.Header().Set("Content-Type", "application/grpc+proto")
	w.Header().Set("grpc-status", fmt.Sprintf("%d", grpcStatusCode(rpcErr.Code)))
	w.Header().Set("grpc-message", rpcErr.Message)
	w.WriteHeader(http.StatusOK)
}

// parseGRPCTimeout parses gRPC timeout format (e.g., "10S" for 10 seconds).
func parseGRPCTimeout(timeout string) (time.Duration, error) {
	if len(timeout) < 2 {
		return 0, fmt.Errorf("invalid timeout format")
	}

	value, err := strconv.ParseInt(timeout[:len(timeout)-1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := timeout[len(timeout)-1]
	switch unit {
	case 'H': // hours
		return time.Duration(value) * time.Hour, nil
	case 'M': // minutes
		return time.Duration(value) * time.Minute, nil
	case 'S': // seconds
		return time.Duration(value) * time.Second, nil
	case 'm': // milliseconds
		return time.Duration(value) * time.Millisecond, nil
	case 'u': // microseconds
		return time.Duration(value) * time.Microsecond, nil
	case 'n': // nanoseconds
		return time.Duration(value), nil
	default:
		return 0, fmt.Errorf("unknown time unit: %c", unit)
	}
}
