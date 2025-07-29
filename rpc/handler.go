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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/i2y/hyperway/codec"
	reflectutil "github.com/i2y/hyperway/internal/reflect"
	"github.com/i2y/hyperway/schema"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

// Context keys.
const (
	contextKeyCancel  contextKey = "cancel"
	handlerContextKey contextKey = "hyperway-handler-context"
)

// Content type constants
const (
	contentTypeConnectJSON = "application/connect+json"
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
	contentTypeJSON         = "application/json"
	contentTypeProtobuf     = "application/protobuf"
	contentTypeXProtobuf    = "application/x-protobuf"
	contentTypeGRPCProto    = "application/grpc+proto"
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

	// Pool for streaming frame buffers
	streamFramePool = sync.Pool{
		New: func() any {
			// Start with 4KB buffer, can grow as needed
			const defaultBufferSize = 4096
			b := make([]byte, 0, defaultBufferSize)
			return &b
		},
	}
)

// handlerContext holds the context for a handler.
type handlerContext struct {
	inputCodec       *codec.Codec
	outputCodec      *codec.Codec
	method           *Method
	validator        interface{ Struct(any) error }
	options          ServiceOptions
	interceptors     []Interceptor
	handlerInfo      *HandlerInfo // Cached handler metadata
	responseHeaders  map[string][]string
	responseTrailers map[string][]string
	requestHeaders   map[string][]string                     // Added to capture request headers
	useProtoInput    bool                                    // Whether to use proto.Message for input
	useProtoOutput   bool                                    // Whether to use proto.Message for output
	handlerFunc      func(context.Context, any) (any, error) // Cached type-erased handler
	newInputFunc     func() reflect.Value                    // Cached function to create new input instance
}

// SetResponseHeader sets a response header.
func (h *handlerContext) SetResponseHeader(key, value string) {
	if h.responseHeaders == nil {
		h.responseHeaders = make(map[string][]string)
	}
	h.responseHeaders[key] = append(h.responseHeaders[key], value)
}

// SetResponseTrailer sets a response trailer.
func (h *handlerContext) SetResponseTrailer(key, value string) {
	if h.responseTrailers == nil {
		h.responseTrailers = make(map[string][]string)
	}
	h.responseTrailers[key] = append(h.responseTrailers[key], value)
}

// GetHandlerContext retrieves the handler context from a context.Context
func GetHandlerContext(ctx context.Context) *handlerContext {
	if hctx, ok := ctx.Value(handlerContextKey).(*handlerContext); ok {
		return hctx
	}
	return nil
}

// GetRequestHeader gets a request header value.
func (h *handlerContext) GetRequestHeader(key string) []string {
	if h.requestHeaders == nil {
		return nil
	}
	return h.requestHeaders[key]
}

// GetRequestHeaders gets all request headers.
func (h *handlerContext) GetRequestHeaders() map[string][]string {
	return h.requestHeaders
}

// createHTTPHandler creates an HTTP handler for a method.
func (s *Service) createHTTPHandler(method *Method) http.HandlerFunc {
	// For streaming methods, create a streaming handler
	if method.StreamType != StreamTypeUnary {
		return s.createStreamingHTTPHandler(method)
	}

	// Prepare handler context once during initialization
	// This caches codec creation and type checking
	cachedCtx, err := s.prepareHandlerContext(method)
	if err != nil {
		// Return error handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.writeError(w, r, err)
		})
	}

	// Cache the prepared context in the service
	s.handlerCtxCache[method.Name] = cachedCtx

	// Create a handler that supports Connect protocol
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get context from pool
		ctx := handlerContextPool.Get().(*handlerContext)

		// Copy cached values instead of recomputing
		ctx.inputCodec = cachedCtx.inputCodec
		ctx.outputCodec = cachedCtx.outputCodec
		ctx.method = cachedCtx.method
		ctx.validator = cachedCtx.validator
		ctx.options = cachedCtx.options
		ctx.handlerInfo = cachedCtx.handlerInfo
		ctx.useProtoInput = cachedCtx.useProtoInput
		ctx.useProtoOutput = cachedCtx.useProtoOutput
		ctx.handlerFunc = cachedCtx.handlerFunc
		ctx.newInputFunc = cachedCtx.newInputFunc

		// Initialize mutable fields
		if ctx.responseHeaders == nil {
			ctx.responseHeaders = make(map[string][]string)
		} else {
			clear(ctx.responseHeaders)
		}
		if ctx.responseTrailers == nil {
			ctx.responseTrailers = make(map[string][]string)
		} else {
			clear(ctx.responseTrailers)
		}
		// Request headers will be set during request processing
		ctx.requestHeaders = nil

		// Copy interceptors
		ctx.interceptors = ctx.interceptors[:0]
		ctx.interceptors = append(ctx.interceptors, cachedCtx.interceptors...)

		// Return context to pool when done
		defer func() {
			// Clear the context before returning to pool
			// Don't set to nil - just clear the maps
			if ctx.responseHeaders != nil {
				clear(ctx.responseHeaders)
			}
			if ctx.responseTrailers != nil {
				clear(ctx.responseTrailers)
			}
			// requestHeaders is just a reference, so set to nil
			ctx.requestHeaders = nil
			handlerContextPool.Put(ctx)
		}()

		s.handleRequest(w, r, ctx)
	})

	// Wrap with Connect protocol support
	// The handler already supports JSON, and Vanguard will handle protocol translation
	return handler
}

// prepareHandlerContext prepares the handler context.
func (s *Service) prepareHandlerContext(method *Method) (*handlerContext, error) {
	var inputCodec, outputCodec *codec.Codec
	var handlerInfo *HandlerInfo

	// For streaming methods, we need different handling
	if method.StreamType != StreamTypeUnary {
		// Streaming methods have InputType and OutputType set by the builder
		// We don't need handler info for streaming

		// Only create codecs if we're not using protobuf types
		if method.ProtoInput == nil || method.ProtoOutput == nil {
			// Build message descriptors (cached in builder)
			inputDesc, err := s.builder.BuildMessage(method.InputType)
			if err != nil {
				return nil, fmt.Errorf("failed to build input descriptor: %w", err)
			}

			outputDesc, err := s.builder.BuildMessage(method.OutputType)
			if err != nil {
				return nil, fmt.Errorf("failed to build output descriptor: %w", err)
			}

			// Create codecs
			inputCodec, err = codec.New(inputDesc, codec.DefaultOptions())
			if err != nil {
				return nil, fmt.Errorf("failed to create input codec: %w", err)
			}

			outputCodec, err = codec.New(outputDesc, codec.DefaultOptions())
			if err != nil {
				return nil, fmt.Errorf("failed to create output codec: %w", err)
			}
		}
	} else {
		// Unary method - use handler info
		var err error
		handlerInfo, err = GetHandlerInfo(method.Handler)
		if err != nil {
			return nil, err
		}

		// Only create codecs if we're not using protobuf types
		if method.ProtoInput == nil || method.ProtoOutput == nil {
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
			inputCodec, err = codec.New(inputDesc, codec.DefaultOptions())
			if err != nil {
				return nil, fmt.Errorf("failed to create input codec: %w", err)
			}

			outputCodec, err = codec.New(outputDesc, codec.DefaultOptions())
			if err != nil {
				return nil, fmt.Errorf("failed to create output codec: %w", err)
			}
		}
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
	ctx.useProtoInput = method.ProtoInput != nil
	ctx.useProtoOutput = method.ProtoOutput != nil

	// Clear headers and trailers
	if ctx.responseHeaders == nil {
		ctx.responseHeaders = make(map[string][]string)
	} else {
		clear(ctx.responseHeaders)
	}
	if ctx.responseTrailers == nil {
		ctx.responseTrailers = make(map[string][]string)
	} else {
		clear(ctx.responseTrailers)
	}
	if ctx.requestHeaders == nil {
		ctx.requestHeaders = make(map[string][]string)
	} else {
		clear(ctx.requestHeaders)
	}

	// Clear and rebuild interceptors slice
	ctx.interceptors = ctx.interceptors[:0]
	// Add method-specific interceptors
	ctx.interceptors = append(ctx.interceptors, method.Options.Interceptors...)
	// Add service-level interceptors
	ctx.interceptors = append(ctx.interceptors, s.options.Interceptors...)

	// Create type-erased handler function to avoid reflection on each call
	if method.StreamType == StreamTypeUnary && handlerInfo != nil {
		ctx.handlerFunc = func(reqCtx context.Context, req any) (any, error) {
			// Use cached handler value for better performance
			results := handlerInfo.HandlerValue.Call([]reflect.Value{
				reflect.ValueOf(reqCtx),
				reflect.ValueOf(req),
			})

			// Check error
			if !results[1].IsNil() {
				return nil, results[1].Interface().(error)
			}

			return results[0].Interface(), nil
		}
	} else {
		// For streaming methods, we don't use handlerFunc
		ctx.handlerFunc = nil
	}

	// Create cached input instance creator
	inputType := method.InputType
	if inputType != nil {
		ctx.newInputFunc = func() reflect.Value {
			return reflect.New(inputType)
		}
	} else {
		// For cases where InputType might not be set yet
		ctx.newInputFunc = func() reflect.Value {
			if method.InputType != nil {
				return reflect.New(method.InputType)
			}
			// This should not happen
			panic("InputType not set for method")
		}
	}

	return ctx, nil
}

// protocolInfo contains information about the request protocol.
type protocolInfo struct {
	isConnect  bool
	isGRPC     bool
	isGRPCWeb  bool
	wantsJSON  bool
	wantsProto bool
}

// detectProtocol detects the protocol type from the request.
func detectProtocol(r *http.Request) protocolInfo {
	contentType := r.Header.Get("Content-Type")
	connectProtocol := r.Header.Get("Connect-Protocol-Version")
	grpcWeb := r.Header.Get("X-Grpc-Web") == "1" || r.Header.Get("grpc-web") == "1"

	info := protocolInfo{
		isConnect: connectProtocol == "1",
		isGRPC:    strings.HasPrefix(contentType, "application/grpc") && !strings.Contains(contentType, "grpc-web"),
		isGRPCWeb: grpcWeb || strings.Contains(contentType, "grpc-web"),
	}

	// For gRPC, check if it's specifically gRPC (not gRPC-Web)
	if info.isGRPC && (grpcWeb || strings.Contains(contentType, "grpc-web")) {
		info.isGRPC = false
		info.isGRPCWeb = true
	}

	// Determine codec preference
	if strings.Contains(contentType, "+json") || strings.Contains(contentType, "/json") {
		info.wantsJSON = true
	} else if strings.Contains(contentType, "+proto") || strings.Contains(contentType, "protobuf") || info.isGRPC {
		info.wantsProto = true
	}

	// Check Accept header as well
	accept := r.Header.Get("Accept")
	if accept != "" && accept != "*/*" {
		if strings.Contains(accept, "+json") || strings.Contains(accept, "/json") {
			info.wantsJSON = true
			info.wantsProto = false
		} else if strings.Contains(accept, "+proto") || strings.Contains(accept, "protobuf") {
			info.wantsProto = true
			info.wantsJSON = false
		}
	}

	// Default to proto for gRPC
	if info.isGRPC && !info.wantsJSON {
		info.wantsProto = true
	}

	return info
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
	// Capture request headers - only copy what we need
	// Instead of copying all headers, just store a reference for lazy access
	ctx.requestHeaders = r.Header

	// Detect protocol
	proto := detectProtocol(r)

	// Only accept POST
	if r.Method != http.MethodPost {
		s.handleMethodNotAllowed(w, r, proto)
		return
	}

	// Check if this is a streaming method
	if ctx.method.StreamType != StreamTypeUnary {
		switch ctx.method.StreamType {
		case StreamTypeServerStream:
			s.handleServerStreamRequest(w, r, ctx, proto)
		case StreamTypeClientStream:
			s.handleClientStreamRequest(w, r, ctx, proto)
		case StreamTypeBidiStream:
			s.handleBidiStreamRequest(w, r, ctx, proto)
		case StreamTypeUnary:
			// This should not happen due to the if condition above
			panic("unreachable: unary stream type in streaming handler")
		}
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

// writeError writes an error response.
func (s *Service) writeError(w http.ResponseWriter, r *http.Request, err error) {
	// Check if this is a Connect protocol request
	connectProtocol := r.Header.Get("Connect-Protocol-Version")
	isConnect := connectProtocol == "1"

	// Convert error to our Error type if needed
	var rpcErr *Error

	// Check error type
	switch e := err.(type) {
	case *ErrorWithDetails:
		// Get the protocol from the request
		protocol := "connect" // Default
		if strings.Contains(r.Header.Get("Content-Type"), "grpc") {
			protocol = "grpc"
		}
		rpcErr = e.ToError(protocol)
	case *Error:
		rpcErr = e
	default:
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
	// Connect protocol always uses HTTP 200 for errors
	w.WriteHeader(http.StatusOK)

	response := map[string]any{
		"code":    string(err.Code),
		"message": err.Message,
	}
	if err.Details != nil {
		// Check if details contains the formatted details
		if details, ok := err.Details["details"]; ok {
			response["details"] = details
		} else {
			// Legacy format - wrap in array
			response["details"] = []any{err.Details}
		}
	}

	// For now, always encode as JSON even for proto requests
	_ = json.NewEncoder(w).Encode(response)
}

// HandlerFunc is the signature for RPC handlers.
type HandlerFunc func(context.Context, any) (any, error)

// decodeInput decodes the input based on content type.
func (s *Service) decodeInput(contentType string, body []byte, ctx *handlerContext) (reflect.Value, error) {
	// If we have a protobuf type, use it directly
	if ctx.useProtoInput && ctx.method.ProtoInput != nil {
		// Clone the proto message to get a fresh instance
		msg := proto.Clone(ctx.method.ProtoInput)

		// Decode based on content type
		switch contentType {
		case contentTypeJSON, contentTypeConnectJSON:
			// Use protojson for proper protobuf JSON handling
			// Use DiscardUnknown option to ignore unknown fields
			unmarshaler := protojson.UnmarshalOptions{
				DiscardUnknown: true,
			}
			if err := unmarshaler.Unmarshal(body, msg); err != nil {
				return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal JSON: %v", err)
			}
		case contentTypeProtobuf, contentTypeXProtobuf, contentTypeProto, contentTypeConnectProto, contentTypeGRPCProto:
			// Direct protobuf unmarshal
			if err := proto.Unmarshal(body, msg); err != nil {
				return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal protobuf: %v", err)
			}
		default:
			// For gRPC, default to protobuf
			if strings.HasPrefix(contentType, "application/grpc") {
				if err := proto.Unmarshal(body, msg); err != nil {
					return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal protobuf: %v", err)
				}
			} else {
				// Default to JSON
				unmarshaler := protojson.UnmarshalOptions{
					DiscardUnknown: true,
				}
				if err := unmarshaler.Unmarshal(body, msg); err != nil {
					return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal: %v", err)
				}
			}
		}

		return reflect.ValueOf(msg), nil
	}

	// Original logic for non-protobuf types
	// Create input instance using cached function
	if ctx.newInputFunc == nil {
		return reflect.Value{}, NewError(CodeInternal, "newInputFunc not initialized")
	}
	inputVal := ctx.newInputFunc()

	// Decode based on content type
	switch contentType {
	case "application/json", contentTypeConnectJSON:
		if err := json.Unmarshal(body, inputVal.Interface()); err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal JSON: %v", err)
		}
	case "application/protobuf", "application/x-protobuf", contentTypeProto, contentTypeConnectProto, contentTypeGRPCProto:
		// Decode protobuf
		if ctx.inputCodec == nil {
			return reflect.Value{}, NewError(CodeInternal, "inputCodec not initialized")
		}
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
		// For gRPC, default to protobuf
		if strings.HasPrefix(contentType, "application/grpc") {
			// Decode protobuf
			if ctx.inputCodec == nil {
				return reflect.Value{}, NewError(CodeInternal, "inputCodec not initialized")
			}
			msg, err := ctx.inputCodec.Unmarshal(body)
			if err != nil {
				return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal protobuf: %v", err)
			}
			defer ctx.inputCodec.ReleaseMessage(msg)

			// Convert to struct
			if err := reflectutil.ProtoToStruct(msg.ProtoReflect(), inputVal.Interface()); err != nil {
				return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to convert proto to struct: %v", err)
			}
		} else {
			// Default to JSON
			if err := json.Unmarshal(body, inputVal.Interface()); err != nil {
				return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal: %v", err)
			}
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
	// Add handler context to the context
	ctx = context.WithValue(ctx, handlerContextKey, hctx)

	// Use cached handler function to avoid reflection
	baseHandler := hctx.handlerFunc

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

	// Set the content-type header first
	w.Header().Set("Content-Type", contentType)

	// Apply response headers from context
	if ctx.responseHeaders != nil {
		for key, values := range ctx.responseHeaders {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Handle trailers
	proto := detectProtocol(r)
	if len(ctx.responseTrailers) > 0 {
		if proto.isConnect {
			// Connect protocol sends trailers as regular headers with "trailer-" prefix
			for key, values := range ctx.responseTrailers {
				for _, value := range values {
					w.Header().Add("trailer-"+key, value)
				}
			}
		} else {
			// gRPC and gRPC-Web use HTTP trailers
			trailerKeys := make([]string, 0, len(ctx.responseTrailers))
			for key := range ctx.responseTrailers {
				trailerKeys = append(trailerKeys, key)
			}
			w.Header().Set("Trailer", strings.Join(trailerKeys, ", "))
		}
	}

	// Handle different content types
	var err error
	if isProtobufContentType(contentType) {
		err = s.encodeProtobufResponse(w, output, ctx, canCompress)
	} else {
		// Default to JSON
		err = s.encodeJSONResponse(w, output, canCompress)
	}

	// Apply trailers after body is written (for non-Connect protocols)
	if ctx.responseTrailers != nil && !proto.isConnect {
		for key, values := range ctx.responseTrailers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	return err
}

// determineContentType determines the response content type
func determineContentType(r *http.Request) string {
	p := detectProtocol(r)

	// Handle gRPC-Web
	if p.isGRPCWeb {
		if p.wantsJSON {
			return "application/grpc-web+json"
		}
		return "application/grpc-web+proto"
	}

	// Handle gRPC
	if p.isGRPC {
		if p.wantsJSON {
			return "application/grpc+json"
		}
		return contentTypeGRPCProto
	}

	// Handle Connect
	if p.isConnect {
		if p.wantsJSON {
			return "application/json"
		}
		return "application/proto"
	}

	// Default based on Accept header
	accept := r.Header.Get("Accept")
	if accept != "" && accept != "*/*" {
		return accept
	}

	// Default based on Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		return contentType
	}

	// Ultimate default
	return contentTypeJSON
}

// isProtobufContentType checks if the content type is protobuf
func isProtobufContentType(contentType string) bool {
	return contentType == "application/protobuf" ||
		contentType == "application/x-protobuf" ||
		contentType == contentTypeProto ||
		contentType == contentTypeConnectProto ||
		contentType == "application/proto" ||
		contentType == contentTypeGRPCProto ||
		contentType == "application/grpc-web+proto" ||
		strings.Contains(contentType, "+proto") ||
		strings.Contains(contentType, "protobuf")
}

// encodeProtobufResponse encodes a protobuf response
func (s *Service) encodeProtobufResponse(w http.ResponseWriter, output any, ctx *handlerContext, canCompress bool) error {
	var data []byte
	var err error

	// Check if output is already a proto.Message
	if msg, ok := output.(proto.Message); ok && ctx.useProtoOutput {
		// Direct protobuf marshal
		data, err = proto.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal protobuf: %w", err)
		}
	} else {
		// Encode struct to protobuf using codec
		data, err = ctx.outputCodec.MarshalStruct(output)
		if err != nil {
			return fmt.Errorf("failed to marshal struct to protobuf: %w", err)
		}
	}

	// Apply compression if needed
	data = s.maybeCompress(data, w, canCompress)

	// Content-Type is already set by encodeResponse
	_, _ = w.Write(data)
	return nil
}

// encodeJSONResponse encodes a JSON response
func (s *Service) encodeJSONResponse(w http.ResponseWriter, output any, canCompress bool) error {
	var data []byte
	var err error

	// Check if output is a proto.Message - use protojson for better compatibility
	if msg, ok := output.(proto.Message); ok {
		// Use protojson for proper JSON encoding of protobuf messages
		data, err = protojson.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal protobuf to JSON: %w", err)
		}
	} else {
		// Standard JSON marshal
		data, err = json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	}

	// Apply compression if needed
	data = s.maybeCompress(data, w, canCompress)

	// Content-Type is already set by encodeResponse
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
	p := detectProtocol(r)
	inputVal, err := s.decodeGRPCInput(message, ctx, p.wantsJSON)
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

// decodeGRPCInput decodes gRPC input.
func (s *Service) decodeGRPCInput(data []byte, ctx *handlerContext, isJSON bool) (reflect.Value, error) {
	// Create input instance
	inputVal := reflect.New(ctx.method.InputType)

	if isJSON {
		// Decode JSON
		if err := json.Unmarshal(data, inputVal.Interface()); err != nil {
			return reflect.Value{}, NewErrorf(CodeInvalidArgument, "failed to unmarshal JSON: %v", err)
		}
	} else {
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
	}

	return inputVal, nil
}

// encodeGRPCResponse encodes and sends a gRPC response.
func (s *Service) encodeGRPCResponse(w http.ResponseWriter, r *http.Request, output any, ctx *handlerContext) error {
	// Determine content type based on request
	p := detectProtocol(r)
	contentType := contentTypeGRPCProto
	if p.wantsJSON {
		contentType = "application/grpc+json"
	}

	// Set gRPC headers
	w.Header().Set("Content-Type", contentType)
	// Declare trailers that will be sent
	w.Header().Set("Trailer", "grpc-status, grpc-message")
	w.WriteHeader(http.StatusOK)

	// Encode struct based on content type
	var data []byte
	var err error
	if p.wantsJSON {
		// Encode as JSON for gRPC+JSON
		data, err = json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal struct to JSON: %w", err)
		}
	} else {
		// Encode as protobuf
		data, err = ctx.outputCodec.MarshalStruct(output)
		if err != nil {
			return fmt.Errorf("failed to marshal struct to protobuf: %w", err)
		}
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

	w.Header().Set("Content-Type", contentTypeGRPCProto)
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

// createStreamingHTTPHandler creates an HTTP handler for streaming methods.
func (s *Service) createStreamingHTTPHandler(method *Method) http.HandlerFunc {
	// Prepare handler context once during initialization
	cachedCtx, err := s.prepareHandlerContext(method)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.writeError(w, r, err)
		})
	}

	// Cache the prepared context
	s.handlerCtxCache[method.Name] = cachedCtx

	return func(w http.ResponseWriter, r *http.Request) {
		// Get context from pool
		ctx := handlerContextPool.Get().(*handlerContext)
		defer func() {
			if ctx.responseHeaders != nil {
				clear(ctx.responseHeaders)
			}
			if ctx.responseTrailers != nil {
				clear(ctx.responseTrailers)
			}
			ctx.requestHeaders = nil
			handlerContextPool.Put(ctx)
		}()

		// Copy cached values
		ctx.inputCodec = cachedCtx.inputCodec
		ctx.outputCodec = cachedCtx.outputCodec
		ctx.method = cachedCtx.method
		ctx.validator = cachedCtx.validator
		ctx.options = cachedCtx.options
		ctx.handlerInfo = cachedCtx.handlerInfo
		ctx.useProtoInput = cachedCtx.useProtoInput
		ctx.useProtoOutput = cachedCtx.useProtoOutput
		ctx.newInputFunc = cachedCtx.newInputFunc
		ctx.handlerFunc = cachedCtx.handlerFunc

		// Initialize mutable fields
		if ctx.responseHeaders == nil {
			ctx.responseHeaders = make(map[string][]string)
		} else {
			clear(ctx.responseHeaders)
		}
		if ctx.responseTrailers == nil {
			ctx.responseTrailers = make(map[string][]string)
		} else {
			clear(ctx.responseTrailers)
		}
		ctx.requestHeaders = r.Header

		// Copy interceptors
		ctx.interceptors = ctx.interceptors[:0]
		ctx.interceptors = append(ctx.interceptors, cachedCtx.interceptors...)

		// Detect protocol
		p := detectProtocol(r)

		switch method.StreamType {
		case StreamTypeServerStream:
			s.handleServerStreamRequest(w, r, ctx, p)
		case StreamTypeClientStream:
			s.handleClientStreamRequest(w, r, ctx, p)
		case StreamTypeBidiStream:
			s.handleBidiStreamRequest(w, r, ctx, p)
		case StreamTypeUnary:
			// Should not happen - unary methods have their own handler
			err := NewError(CodeInternal, "Unary method in streaming handler")
			if p.isConnect {
				s.writeConnectError(w, r, err)
			} else if p.isGRPC {
				s.writeGRPCError(w, err)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		default:
			err := NewError(CodeUnimplemented, "Unknown streaming type")
			if p.isConnect {
				s.writeConnectError(w, r, err)
			} else if p.isGRPC {
				s.writeGRPCError(w, err)
			} else {
				http.Error(w, err.Error(), http.StatusNotImplemented)
			}
		}
	}
}
