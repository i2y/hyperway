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

	// Read message body
	body := make([]byte, messageLength)
	if _, err := io.ReadFull(r.Body, body); err != nil {
		s.writeGRPCError(w, NewError(CodeInternal, "failed to read gRPC message body"))
		return nil, err
	}

	return body, nil
}

// readNonGRPCBody reads a non-gRPC request body
func (s *Service) readNonGRPCBody(r *http.Request, p protocolInfo, w http.ResponseWriter) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("failed to read body: %w", err))
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
	if encoding := r.Header.Get("Content-Encoding"); encoding == CompressionGzip {
		compressor, ok := GetCompressor(CompressionGzip)
		if !ok {
			s.writeError(w, r, fmt.Errorf("gzip decompression not available"))
			return nil, fmt.Errorf("gzip decompression not available")
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
func (s *Service) handleClientStreamRequest(w http.ResponseWriter, r *http.Request, _ *handlerContext, p protocolInfo) {
	// For now, return unimplemented
	err := NewError(CodeUnimplemented, "Client streaming not yet implemented")
	switch {
	case p.isConnect:
		s.writeConnectError(w, r, err)
	case p.isGRPC:
		s.writeGRPCError(w, err)
	default:
		http.Error(w, err.Error(), http.StatusNotImplemented)
	}
}

// handleBidiStreamRequest handles bidirectional streaming RPC requests
func (s *Service) handleBidiStreamRequest(w http.ResponseWriter, r *http.Request, _ *handlerContext, p protocolInfo) {
	// For now, return unimplemented
	err := NewError(CodeUnimplemented, "Bidirectional streaming not yet implemented")
	switch {
	case p.isConnect:
		s.writeConnectError(w, r, err)
	case p.isGRPC:
		s.writeGRPCError(w, err)
	default:
		http.Error(w, err.Error(), http.StatusNotImplemented)
	}
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
}

func newServerStreamWriter(w http.ResponseWriter, r *http.Request, ctx *handlerContext, p protocolInfo) *serverStreamWriter {
	flusher, _ := w.(http.Flusher)
	s := &serverStreamWriter{
		w:           w,
		r:           r,
		ctx:         ctx,
		protocol:    p,
		flusher:     flusher,
		flushPeriod: defaultFlushInterval, // Flush every 10ms or after each message in low-throughput scenarios
		lastFlush:   time.Now(),
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

	// Write the message based on protocol
	var writeErr error
	switch {
	case s.protocol.isConnect:
		writeErr = s.sendConnectMessage(data)
	case s.protocol.isGRPC:
		writeErr = s.sendGRPCMessage(data)
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
		// Don't set Transfer-Encoding explicitly - Go will handle it automatically
	} else if s.protocol.isGRPC {
		ct := determineContentType(s.r)
		s.w.Header().Set("Content-Type", ct)
		s.w.Header().Set("grpc-accept-encoding", "gzip")
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

func (s *serverStreamWriter) sendConnectMessage(data []byte) error {
	// Connect uses a simple length-prefixed format for streaming
	// Format: 1 byte flags + 4 bytes length (big-endian) + data

	// Get a frame buffer from pool
	frameSize := frameHeaderLength + len(data)
	frameBuf := s.getFrameBuffer(frameSize)
	defer s.putFrameBuffer(frameBuf)

	// Build frame in single buffer
	frame := (*frameBuf)[:frameSize]
	frame[0] = 0                                                                            // flags (0 = no compression)
	binary.BigEndian.PutUint32(frame[frameLengthOffset:frameLengthSize], uint32(len(data))) //nolint:gosec // length is bounded by message size limits
	copy(frame[frameHeaderLength:], data)

	// Single write for entire frame
	if _, err := s.w.Write(frame); err != nil {
		return err
	}

	// Smart flushing: flush if enough time has passed since last flush
	// This balances latency and throughput
	if s.flusher != nil && time.Since(s.lastFlush) >= s.flushPeriod {
		s.flusher.Flush()
		s.lastFlush = time.Now()
	}

	return nil
}

func (s *serverStreamWriter) sendGRPCMessage(data []byte) error {
	// gRPC frame format: 1 byte flags + 4 bytes length + data
	frameSize := frameHeaderLength + len(data)
	frameBuf := s.getFrameBuffer(frameSize)
	defer s.putFrameBuffer(frameBuf)

	frame := (*frameBuf)[:frameSize]

	// Flags (0 = no compression)
	frame[0] = 0

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
	if s.flusher != nil && time.Since(s.lastFlush) >= s.flushPeriod {
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
