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
	"time"

	"google.golang.org/protobuf/proto"
)

// Constants
const (
	frameHeaderLength    = 5
	frameLengthOffset    = 1
	frameLengthSize      = 5
	defaultFlushInterval = 10 * time.Millisecond
)

// handleServerStreamRequest handles server-streaming RPC requests
func (s *Service) handleServerStreamRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo) {
	// Add panic recovery
	defer func() {
		if p := recover(); p != nil {
			err := fmt.Errorf("panic in streaming handler: %v", p)
			s.writeError(w, r, err)
		}
	}()

	// Only accept POST
	if r.Method != http.MethodPost {
		s.handleMethodNotAllowed(w, r, p)
		return
	}

	// Parse timeout
	reqCtx := parseRequestTimeout(r, p.isConnect)
	if cancel, ok := reqCtx.Value(contextKeyCancel).(context.CancelFunc); ok {
		defer cancel()
		reqCtx = context.WithValue(reqCtx, contextKeyCancel, nil)
	}

	// Read and process request body
	body, err := s.readStreamRequestBody(r, p, w)
	if err != nil {
		return // Error already written
	}

	// Decompress if needed
	body, err = s.decompressRequestBody(r, body, w)
	if err != nil {
		return // Error already written
	}

	// Process the request
	s.processStreamRequest(w, r, ctx, p, body, reqCtx)
}

// readStreamRequestBody reads the request body based on protocol
func (s *Service) readStreamRequestBody(r *http.Request, p protocolInfo, w http.ResponseWriter) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()

	if p.isGRPC {
		return s.readGRPCFramedBody(r, p, w)
	}
	return s.readNonGRPCBody(r, p, w)
}

// readGRPCFramedBody reads a gRPC framed message
func (s *Service) readGRPCFramedBody(r *http.Request, _ protocolInfo, w http.ResponseWriter) ([]byte, error) {
	frameHeader := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(r.Body, frameHeader); err != nil {
		s.writeGRPCError(w, NewError(CodeInternal, "failed to read gRPC frame header"))
		return nil, err
	}

	// Parse frame header
	messageLength := binary.BigEndian.Uint32(frameHeader[frameLengthOffset:frameLengthSize])

	// Check if message size exceeds limit
	if int(messageLength) > s.options.MaxReceiveMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("gRPC message size %d exceeds maximum allowed size %d",
				messageLength, s.options.MaxReceiveMessageSize))
		s.writeGRPCError(w, err)
		return nil, err
	}

	// Read message body
	body := make([]byte, messageLength)
	if _, err := io.ReadFull(r.Body, body); err != nil {
		s.writeGRPCError(w, NewError(CodeInternal, "failed to read gRPC message body"))
		return nil, err
	}

	// Check if compressed and validate decompressed size
	compressionFlag := frameHeader[0]
	if compressionFlag == 1 {
		// Get compression type from headers
		encoding := r.Header.Get("grpc-encoding")
		if encoding != "" && encoding != encodingIdentity {
			decompressed, err := s.decompressBodyWithType(body, encoding)
			if err != nil {
				s.writeGRPCError(w, NewError(CodeInternal, "failed to decompress gRPC message"))
				return nil, err
			}
			// Check decompressed size
			if len(decompressed) > s.options.MaxReceiveMessageSize {
				err := NewError(CodeResourceExhausted,
					fmt.Sprintf("decompressed gRPC message size %d exceeds maximum allowed size %d",
						len(decompressed), s.options.MaxReceiveMessageSize))
				s.writeGRPCError(w, err)
				return nil, err
			}
			return decompressed, nil
		}
	}

	return body, nil
}

// readNonGRPCBody reads a non-gRPC request body
func (s *Service) readNonGRPCBody(r *http.Request, p protocolInfo, w http.ResponseWriter) ([]byte, error) {
	// Use LimitReader to enforce max receive message size
	limitedReader := io.LimitReader(r.Body, int64(s.options.MaxReceiveMessageSize)+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("failed to read body: %w", err))
		return nil, err
	}

	// Check if we exceeded the size limit
	if len(body) > s.options.MaxReceiveMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("message size %d exceeds maximum allowed size %d",
				len(body), s.options.MaxReceiveMessageSize))
		s.writeError(w, r, err)
		return nil, err
	}

	// Check if this is a Connect protocol request with framing
	if p.isConnect && len(body) >= frameHeaderLength {
		// Check if it looks like Connect framing (5-byte header)
		length := binary.BigEndian.Uint32(body[frameLengthOffset:frameLengthSize])
		if int(length) == len(body)-frameHeaderLength {
			// This is a framed message, extract the actual message
			body = body[frameHeaderLength:]
		}
	}

	return body, nil
}

// decompressRequestBody decompresses the request body if needed
func (s *Service) decompressRequestBody(r *http.Request, body []byte, w http.ResponseWriter) ([]byte, error) {
	encoding := r.Header.Get("Content-Encoding")
	if encoding != "" && encoding != "identity" {
		compressor, ok := GetCompressor(encoding)
		if !ok {
			err := fmt.Errorf("%s decompression not available", encoding)
			s.writeError(w, r, err)
			return nil, err
		}
		decompressed, err := compressor.Decompress(body)
		if err != nil {
			s.writeError(w, r, fmt.Errorf("failed to decompress request: %w", err))
			return nil, err
		}
		return decompressed, nil
	}
	return body, nil
}

// processStreamRequest processes the streaming request
func (s *Service) processStreamRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo, body []byte, reqCtx context.Context) {
	// Decode input
	inputVal, decodeErr := s.decodeInput(r.Header.Get("Content-Type"), body, ctx)
	if decodeErr != nil {
		s.writeProtocolError(w, r, p, decodeErr)
		return
	}

	// Validate if enabled
	if err := s.validateInput(inputVal, ctx); err != nil {
		s.writeProtocolError(w, r, p, err)
		return
	}

	// Create stream implementation
	baseStream := newServerStreamWriter(w, r, ctx, p)

	// Add handler context to the request context
	reqCtx = context.WithValue(reqCtx, handlerContextKey, ctx)

	// Call the handler
	if err := s.callStreamHandler(ctx, reqCtx, inputVal, baseStream); err != nil {
		baseStream.sendError(err)
		return
	}

	// Finalize the stream
	baseStream.finalize()
}

// callClientStreamHandler calls the client streaming handler
func (s *Service) callClientStreamHandler(ctx *handlerContext, reqCtx context.Context, reader *clientStreamReader) (any, error) {
	// Type assert to the wrapped handler signature
	if wrappedHandler, ok := ctx.method.Handler.(func(context.Context, any) (any, error)); ok {
		// Call the wrapped handler
		return wrappedHandler(reqCtx, reader)
	}

	// Fallback to reflection
	handlerValue := reflect.ValueOf(ctx.method.Handler)
	results := handlerValue.Call([]reflect.Value{
		reflect.ValueOf(reqCtx),
		reflect.ValueOf(reader),
	})

	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

// sendClientStreamResponse sends the response for client streaming
func (s *Service) sendClientStreamResponse(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo, output any) {
	// Encode the output
	var data []byte
	var err error

	// Determine encoding based on protocol
	if p.wantsJSON || (p.isConnect && r.Header.Get("Content-Type") == "application/json") {
		data, err = json.Marshal(output)
	} else if ctx.useProtoOutput {
		if protoMsg, ok := output.(proto.Message); ok {
			data, err = proto.Marshal(protoMsg)
		} else {
			err = fmt.Errorf("expected proto.Message, got %T", output)
		}
	} else {
		data, err = ctx.outputCodec.MarshalStruct(output)
	}

	if err != nil {
		s.writeProtocolError(w, r, p, NewError(CodeInternal, fmt.Sprintf("failed to encode response: %v", err)))
		return
	}

	// Check message size
	if len(data) > ctx.options.MaxSendMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("response size %d exceeds maximum send size %d",
				len(data), ctx.options.MaxSendMessageSize))
		s.writeProtocolError(w, r, p, err)
		return
	}

	// Apply compression if needed and client accepts it
	var compressed bool
	var compressionType string
	if acceptEncoding := s.getAcceptEncoding(r, p); acceptEncoding != "" {
		if compressor, encoding := selectCompressor(acceptEncoding); compressor != nil && shouldCompress(data) {
			if compressedData, err := compressor.Compress(data); err == nil {
				data = compressedData
				compressed = true
				compressionType = encoding
			}
		}
	}

	// Send response based on protocol
	switch {
	case p.isConnect:
		s.sendConnectClientStreamResponse(w, r, data, compressed, compressionType)
	case p.isGRPC:
		s.sendGRPCClientStreamResponse(w, r, data, compressed, compressionType)
	default:
		// Plain HTTP response
		if compressed {
			w.Header().Set("Content-Encoding", compressionType)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

// getAcceptEncoding gets the accept-encoding header based on protocol
func (s *Service) getAcceptEncoding(r *http.Request, p protocolInfo) string {
	if p.isGRPC || p.isGRPCWeb {
		if enc := r.Header.Get("grpc-accept-encoding"); enc != "" {
			return enc
		}
		return r.Header.Get("grpc-encoding")
	} else if p.isConnect {
		if enc := r.Header.Get("Connect-Accept-Encoding"); enc != "" {
			return enc
		}
		return r.Header.Get("Accept-Encoding")
	}
	return r.Header.Get("Accept-Encoding")
}

// sendConnectClientStreamResponse sends a Connect protocol response for client streaming
func (s *Service) sendConnectClientStreamResponse(w http.ResponseWriter, r *http.Request, data []byte, compressed bool, compressionType string) {
	// Set headers
	contentType := "application/connect+proto"
	if r.Header.Get("Content-Type") == "application/json" {
		contentType = "application/connect+json"
	}
	w.Header().Set("Content-Type", contentType)

	if compressed && compressionType != "" {
		w.Header().Set("Connect-Content-Encoding", compressionType)
	}

	// Apply custom headers
	if ctx, ok := r.Context().Value(handlerContextKey).(*handlerContext); ok && ctx.responseHeaders != nil {
		for key, values := range ctx.responseHeaders {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	w.WriteHeader(http.StatusOK)

	// Connect uses framing for responses too
	// First write the message frame
	frame := make([]byte, frameHeaderLength+len(data))
	if compressed {
		frame[0] = 1 // compression flag
	} else {
		frame[0] = 0
	}
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits
	copy(frame[5:], data)

	_, _ = w.Write(frame)

	// Send end-of-stream frame for Connect protocol
	// Connect expects an empty JSON object {} in the end frame
	endPayload := []byte("{}")
	endFrame := make([]byte, frameHeaderLength+len(endPayload))
	endFrame[0] = 0x02 // end-of-stream flag
	binary.BigEndian.PutUint32(endFrame[1:5], uint32(len(endPayload)))
	copy(endFrame[5:], endPayload)
	_, _ = w.Write(endFrame)
}

// sendGRPCClientStreamResponse sends a gRPC protocol response for client streaming
func (s *Service) sendGRPCClientStreamResponse(w http.ResponseWriter, r *http.Request, data []byte, compressed bool, compressionType string) {
	// Set headers
	ct := determineContentType(r)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("grpc-accept-encoding", "gzip, br, zstd")

	if compressed && compressionType != "" {
		w.Header().Set("grpc-encoding", compressionType)
	}

	// Apply custom headers
	if ctx, ok := r.Context().Value(handlerContextKey).(*handlerContext); ok && ctx.responseHeaders != nil {
		for key, values := range ctx.responseHeaders {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Set trailers
	w.Header().Set("Trailer", "grpc-status, grpc-message")
	w.WriteHeader(http.StatusOK)

	// Write gRPC frame
	frame := make([]byte, frameHeaderLength+len(data))
	if compressed {
		frame[0] = 1 // compression flag
	} else {
		frame[0] = 0
	}
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits
	copy(frame[5:], data)

	_, _ = w.Write(frame)

	// Send success trailers
	trailer := w.Header()
	trailer.Set("grpc-status", "0")
	trailer.Set("grpc-message", "")

	// Apply custom trailers
	if ctx, ok := r.Context().Value(handlerContextKey).(*handlerContext); ok && ctx.responseTrailers != nil {
		for key, values := range ctx.responseTrailers {
			for _, value := range values {
				trailer.Add(key, value)
			}
		}
	}
}

// writeProtocolError writes an error based on the protocol
func (s *Service) writeProtocolError(w http.ResponseWriter, r *http.Request, p protocolInfo, err error) {
	if p.isGRPC {
		s.writeGRPCError(w, err.(*Error))
	} else {
		s.writeError(w, r, err)
	}
}

// callStreamHandler calls the streaming handler
func (s *Service) callStreamHandler(ctx *handlerContext, reqCtx context.Context, inputVal reflect.Value, baseStream *serverStreamWriter) error {
	// Type assert to the wrapped handler signature
	if wrappedHandler, ok := ctx.method.Handler.(func(context.Context, any, any) error); ok {
		// Call the wrapped handler
		return wrappedHandler(reqCtx, inputVal.Interface(), baseStream)
	}

	// Fallback to reflection
	handlerValue := reflect.ValueOf(ctx.method.Handler)
	results := handlerValue.Call([]reflect.Value{
		reflect.ValueOf(reqCtx),
		inputVal,
		reflect.ValueOf(baseStream),
	})

	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

// handleClientStreamRequest handles client-streaming RPC requests
func (s *Service) handleClientStreamRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo) {
	// Add panic recovery
	defer func() {
		if p := recover(); p != nil {
			err := fmt.Errorf("panic in client streaming handler: %v", p)
			s.writeError(w, r, err)
		}
	}()

	// Only accept POST
	if r.Method != http.MethodPost {
		s.handleMethodNotAllowed(w, r, p)
		return
	}

	// Parse timeout
	reqCtx := parseRequestTimeout(r, p.isConnect)
	if cancel, ok := reqCtx.Value(contextKeyCancel).(context.CancelFunc); ok {
		defer cancel()
		reqCtx = context.WithValue(reqCtx, contextKeyCancel, nil)
	}

	// Create client stream reader
	reader := newClientStreamReader(r, ctx, p)
	defer func() { _ = reader.Close() }()

	// Add handler context to the request context
	reqCtx = context.WithValue(reqCtx, handlerContextKey, ctx)

	// Call the client streaming handler
	output, err := s.callClientStreamHandler(ctx, reqCtx, reader)
	if err != nil {
		s.writeProtocolError(w, r, p, err)
		return
	}

	// Send the response
	s.sendClientStreamResponse(w, r, ctx, p, output)
}

// handleBidiStreamRequest handles bidirectional streaming RPC requests
func (s *Service) handleBidiStreamRequest(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo) {
	// Add panic recovery
	defer func() {
		if p := recover(); p != nil {
			err := fmt.Errorf("panic in bidi streaming handler: %v", p)
			s.writeError(w, r, err)
		}
	}()

	// Only accept POST
	if r.Method != http.MethodPost {
		s.handleMethodNotAllowed(w, r, p)
		return
	}

	// Parse timeout
	reqCtx := parseRequestTimeout(r, p.isConnect)
	if cancel, ok := reqCtx.Value(contextKeyCancel).(context.CancelFunc); ok {
		defer cancel()
		reqCtx = context.WithValue(reqCtx, contextKeyCancel, nil)
	}

	// Create bidirectional stream
	stream := newBidiStream(r, w, ctx, p)
	defer func() { _ = stream.Close() }()

	// Add handler context to the request context
	reqCtx = context.WithValue(reqCtx, handlerContextKey, ctx)

	// Send initial headers for gRPC protocol
	if p.isGRPC {
		stream.writer.sendHeaders()
		stream.writer.headersSent = true
		// Flush immediately to establish the stream
		if stream.writer.flusher != nil {
			stream.writer.flusher.Flush()
		}
	}

	// Call the bidirectional streaming handler
	if err := s.callBidiStreamHandler(ctx, reqCtx, stream); err != nil {
		stream.writer.sendError(err)
		return
	}

	// Finalize the stream
	stream.writer.finalize()
}

// callBidiStreamHandler calls the bidirectional streaming handler
func (s *Service) callBidiStreamHandler(ctx *handlerContext, reqCtx context.Context, stream *bidiStream) error {
	// Type assert to the wrapped handler signature
	if wrappedHandler, ok := ctx.method.Handler.(func(context.Context, any) error); ok {
		// Call the wrapped handler
		return wrappedHandler(reqCtx, stream)
	}

	// Fallback to reflection
	handlerValue := reflect.ValueOf(ctx.method.Handler)
	results := handlerValue.Call([]reflect.Value{
		reflect.ValueOf(reqCtx),
		reflect.ValueOf(stream),
	})

	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

// serverStreamWriter implements server-side streaming
type serverStreamWriter struct {
	w            http.ResponseWriter
	r            *http.Request
	ctx          *handlerContext
	protocol     protocolInfo
	headersSent  bool
	mu           sync.Mutex
	err          error
	messageCount int
	flusher      http.Flusher
	connectEnded bool

	// Cached encoding function to avoid repeated checks
	encodeFunc func(any) ([]byte, error)

	// Batching control
	lastFlush   time.Time
	flushPeriod time.Duration

	// Compression settings
	compressor      Compressor
	compressionType string // e.g., "gzip", "br", "zstd"
	canCompress     bool
	shouldCompress  bool // whether to actually compress messages
}

func newServerStreamWriter(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo) *serverStreamWriter {
	flusher, _ := w.(http.Flusher)

	// For bidirectional streaming, flush immediately after each message
	flushPeriod := defaultFlushInterval
	if ctx != nil && ctx.method != nil && ctx.method.StreamType == StreamTypeBidiStream {
		flushPeriod = 0 // Immediate flush for bidi streams
	}

	s := &serverStreamWriter{
		w:           w,
		r:           r,
		ctx:         ctx,
		protocol:    p,
		flusher:     flusher,
		flushPeriod: flushPeriod,
		lastFlush:   time.Now(),
	}

	// Setup compression if client accepts it
	var acceptEncoding string
	if p.isGRPC || p.isGRPCWeb {
		acceptEncoding = r.Header.Get("grpc-accept-encoding")
		if acceptEncoding == "" {
			acceptEncoding = r.Header.Get("grpc-encoding")
		}
	} else if p.isConnect {
		acceptEncoding = r.Header.Get("Connect-Accept-Encoding")
		if acceptEncoding == "" {
			acceptEncoding = r.Header.Get("Accept-Encoding")
		}
	}

	// Select best compressor based on client preferences
	if acceptEncoding != "" {
		if c, encoding := selectCompressor(acceptEncoding); c != nil {
			s.compressor = c
			s.compressionType = encoding
			s.shouldCompress = true
			s.canCompress = true
		}
	}

	// Pre-determine encoding function based on protocol
	isJSON := p.wantsJSON
	switch {
	case p.isGRPC && !isJSON:
		// gRPC protobuf encoding
		s.encodeFunc = func(msg any) ([]byte, error) {
			return ctx.outputCodec.MarshalStruct(msg)
		}
	case ctx.useProtoOutput && !isJSON:
		// Connect protobuf encoding
		s.encodeFunc = func(msg any) ([]byte, error) {
			if protoMsg, ok := msg.(proto.Message); ok {
				return proto.Marshal(protoMsg)
			}
			return nil, fmt.Errorf("expected proto.Message, got %T", msg)
		}
	case isJSON:
		// JSON encoding
		s.encodeFunc = json.Marshal
	default:
		// Default: use codec
		s.encodeFunc = func(msg any) ([]byte, error) {
			return ctx.outputCodec.MarshalStruct(msg)
		}
	}

	return s
}

// Context returns the stream context
func (s *serverStreamWriter) Context() context.Context {
	return s.r.Context()
}

// Send sends a message to the client
func (s *serverStreamWriter) Send(msg any) error {
	// Check error state with minimal lock
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return s.err
	}

	// Send headers on first message
	if !s.headersSent {
		s.sendHeaders()
		s.headersSent = true
	}
	s.mu.Unlock()

	// Encode the message outside of lock
	data, err := s.encodeFunc(msg)
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return err
	}

	// Check if encoded message exceeds send size limit
	if len(data) > s.ctx.options.MaxSendMessageSize {
		err := NewError(CodeResourceExhausted,
			fmt.Sprintf("message size %d exceeds maximum send size %d",
				len(data), s.ctx.options.MaxSendMessageSize))
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return err
	}

	// Compress if needed and size threshold met
	compressed := false
	if s.shouldCompress && shouldCompress(data) {
		compressedData, err := s.compressor.Compress(data)
		if err == nil {
			data = compressedData
			compressed = true
		}
	}

	// Write the message based on protocol
	var writeErr error
	switch {
	case s.protocol.isConnect:
		writeErr = s.sendConnectMessage(data, compressed)
	case s.protocol.isGRPC:
		writeErr = s.sendGRPCMessage(data, compressed)
	default:
		// Plain HTTP streaming (newline-delimited JSON)
		_, writeErr = s.w.Write(data)
		if writeErr == nil {
			_, writeErr = s.w.Write([]byte("\n"))
		}
		if writeErr == nil && s.flusher != nil {
			s.flusher.Flush()
		}
	}

	// Update state with lock
	if writeErr != nil {
		s.mu.Lock()
		s.err = writeErr
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.messageCount++
		s.mu.Unlock()
	}

	return writeErr
}

func (s *serverStreamWriter) sendHeaders() {
	// Set appropriate headers based on protocol
	if s.protocol.isConnect {
		// For Connect streaming, use application/connect+json or application/connect+proto
		contentType := "application/connect+proto"
		if s.protocol.wantsJSON {
			contentType = "application/connect+json"
		}
		s.w.Header().Set("Content-Type", contentType)
		s.w.Header().Set("Cache-Control", "no-cache")
		// Indicate compression support for Connect
		if s.shouldCompress && s.compressionType != "" {
			s.w.Header().Set("Connect-Content-Encoding", s.compressionType)
		}
		// Don't set Transfer-Encoding explicitly - Go will handle it automatically
	} else if s.protocol.isGRPC {
		ct := determineContentType(s.r)
		s.w.Header().Set("Content-Type", ct)
		s.w.Header().Set("grpc-accept-encoding", "gzip, br, zstd")
		// Indicate if we're using compression
		if s.shouldCompress && s.compressionType != "" {
			s.w.Header().Set("grpc-encoding", s.compressionType)
		}
		s.w.Header().Set("Trailer", "grpc-status, grpc-message")
	}

	// Apply custom headers
	if s.ctx.responseHeaders != nil {
		for key, values := range s.ctx.responseHeaders {
			for _, value := range values {
				s.w.Header().Add(key, value)
			}
		}
	}

	// Write status - Connect also needs explicit status
	s.w.WriteHeader(http.StatusOK)
}

func (s *serverStreamWriter) sendConnectMessage(data []byte, compressed bool) error {
	// Connect uses a simple length-prefixed format for streaming
	// Format: 1 byte flags + 4 bytes length (big-endian) + data

	// Get a frame buffer from pool
	frameSize := frameHeaderLength + len(data)
	frameBuf := s.getFrameBuffer(frameSize)
	defer s.putFrameBuffer(frameBuf)

	// Build frame in single buffer
	frame := (*frameBuf)[:frameSize]
	// Set compression flag if message is compressed
	if compressed {
		frame[0] = 1 // compression flag
	} else {
		frame[0] = 0 // no compression
	}
	binary.BigEndian.PutUint32(frame[frameLengthOffset:frameLengthSize], uint32(len(data))) //nolint:gosec // length is bounded by message size limits
	copy(frame[frameHeaderLength:], data)

	// Single write for entire frame
	if _, err := s.w.Write(frame); err != nil {
		return err
	}

	// Smart flushing: flush if enough time has passed since last flush
	// This balances latency and throughput
	// For bidi streams (flushPeriod=0), flush immediately
	if s.flusher != nil && (s.flushPeriod == 0 || time.Since(s.lastFlush) >= s.flushPeriod) {
		s.flusher.Flush()
		s.lastFlush = time.Now()
	}

	return nil
}

func (s *serverStreamWriter) sendGRPCMessage(data []byte, compressed bool) error {
	// gRPC frame format: 1 byte flags + 4 bytes length + data
	frameSize := frameHeaderLength + len(data)
	frameBuf := s.getFrameBuffer(frameSize)
	defer s.putFrameBuffer(frameBuf)

	frame := (*frameBuf)[:frameSize]

	// Set compression flag if message is compressed
	if compressed {
		frame[0] = 1 // compression flag
	} else {
		frame[0] = 0 // no compression
	}

	// Length (big-endian)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits

	// Data
	copy(frame[5:], data)

	// Write frame
	if _, err := s.w.Write(frame); err != nil {
		return err
	}

	// Smart flushing: flush if enough time has passed since last flush
	// This balances latency and throughput
	// For bidi streams (flushPeriod=0), flush immediately
	if s.flusher != nil && (s.flushPeriod == 0 || time.Since(s.lastFlush) >= s.flushPeriod) {
		s.flusher.Flush()
		s.lastFlush = time.Now()
	}

	return nil
}

func (s *serverStreamWriter) sendError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.err = err

	// Convert to RPC error
	var rpcErr *Error
	switch e := err.(type) {
	case *Error:
		rpcErr = e
	case *ErrorWithDetails:
		protocol := protocolConnect
		if s.protocol.isGRPC {
			protocol = protocolGRPC
		}
		rpcErr = e.ToError(protocol)
	default:
		rpcErr = NewError(CodeInternal, err.Error())
	}

	if s.protocol.isConnect {
		// For Connect, send error as final message with end-of-stream marker
		s.sendConnectError(rpcErr)
	} else if s.protocol.isGRPC {
		// For gRPC, errors are sent in trailers
		s.sendGRPCTrailers(rpcErr)
	}
}

func (s *serverStreamWriter) sendConnectError(err *Error) {
	// If headers not sent, send them now
	if !s.headersSent {
		s.sendHeaders()
		s.headersSent = true
	}

	// Connect error format with end-of-stream marker
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

	// Send with end-of-stream flag (0x02)
	if _, err := s.w.Write([]byte{0x02}); err != nil {
		return
	}
	if err := binary.Write(s.w, binary.BigEndian, uint32(len(data))); err != nil { //nolint:gosec // bounded by message size
		return
	}
	if _, err := s.w.Write(data); err != nil {
		return
	}

	if s.flusher != nil {
		s.flusher.Flush()
	}

	s.connectEnded = true
}

func (s *serverStreamWriter) sendGRPCTrailers(err *Error) {
	// gRPC sends errors in HTTP trailers
	trailer := s.w.Header()
	trailer.Set("grpc-status", fmt.Sprintf("%d", grpcStatusCode(err.Code)))
	trailer.Set("grpc-message", err.Message)

	// Apply any custom trailers
	if s.ctx.responseTrailers != nil {
		for key, values := range s.ctx.responseTrailers {
			for _, value := range values {
				trailer.Add(key, value)
			}
		}
	}

	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *serverStreamWriter) finalize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return // Error already sent
	}

	// Send headers if not sent
	if !s.headersSent {
		s.sendHeaders()
		s.headersSent = true
	}

	// Handle protocol-specific finalization
	switch {
	case s.protocol.isConnect && !s.connectEnded:
		s.finalizeConnect()
	case s.protocol.isGRPC:
		s.finalizeGRPC()
	default:
		s.finalizeDefault()
	}
}

// finalizeConnect handles Connect protocol finalization
func (s *serverStreamWriter) finalizeConnect() {
	// Send end-of-stream marker
	if err := s.sendConnectEndOfStream(); err != nil {
		return
	}

	// Apply trailers as headers
	s.applyConnectTrailers()

	// Flush for Connect protocol
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// sendConnectEndOfStream sends the Connect end-of-stream marker
func (s *serverStreamWriter) sendConnectEndOfStream() error {
	endMessage := []byte("{}")
	if _, err := s.w.Write([]byte{0x02}); err != nil { // End-of-stream flag
		return err
	}
	if err := binary.Write(s.w, binary.BigEndian, uint32(len(endMessage))); err != nil { //nolint:gosec // bounded by message size
		return err
	}
	_, err := s.w.Write(endMessage)
	return err
}

// applyConnectTrailers applies trailers as headers with "trailer-" prefix
func (s *serverStreamWriter) applyConnectTrailers() {
	if s.ctx.responseTrailers == nil {
		return
	}
	for key, values := range s.ctx.responseTrailers {
		for _, value := range values {
			s.w.Header().Add("trailer-"+key, value)
		}
	}
}

// finalizeGRPC handles gRPC protocol finalization
func (s *serverStreamWriter) finalizeGRPC() {
	// Set default trailers
	trailer := s.w.Header()
	trailer.Set("grpc-status", "0")
	trailer.Set("grpc-message", "")

	// Apply custom trailers
	s.applyGRPCTrailers(trailer)
	// DO NOT flush for gRPC - let the HTTP/2 transport handle trailer sending
}

// applyGRPCTrailers applies custom trailers for gRPC
func (s *serverStreamWriter) applyGRPCTrailers(trailer http.Header) {
	if s.ctx.responseTrailers == nil {
		return
	}
	for key, values := range s.ctx.responseTrailers {
		for _, value := range values {
			trailer.Add(key, value)
		}
	}
}

// finalizeDefault handles default protocol finalization
func (s *serverStreamWriter) finalizeDefault() {
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// getFrameBuffer gets a buffer from the pool
func (s *serverStreamWriter) getFrameBuffer(size int) *[]byte {
	buf := streamFramePool.Get().(*[]byte)
	if cap(*buf) < size {
		// Need a bigger buffer
		newBuf := make([]byte, size)
		return &newBuf
	}
	// Resize existing buffer
	*buf = (*buf)[:size]
	return buf
}

// putFrameBuffer returns a buffer to the pool
func (s *serverStreamWriter) putFrameBuffer(buf *[]byte) {
	// Reset buffer before returning to pool
	*buf = (*buf)[:0]
	streamFramePool.Put(buf)
}

// Implement typed server stream
type typedServerStream[T any] struct {
	*serverStreamWriter
}

func (s *typedServerStream[T]) Send(msg *T) error {
	return s.serverStreamWriter.Send(msg)
}
