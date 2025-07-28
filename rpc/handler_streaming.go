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

	// Read request body
	var body []byte
	var err error

	if p.isGRPC {
		// gRPC always uses framing
		frameHeader := make([]byte, 5)
		if _, err := io.ReadFull(r.Body, frameHeader); err != nil {
			if p.isGRPC {
				s.writeGRPCError(w, NewError(CodeInternal, "failed to read gRPC frame header"))
			} else {
				s.writeError(w, r, fmt.Errorf("failed to read gRPC frame header: %w", err))
			}
			return
		}

		// Parse frame header
		// compressed := frameHeader[0] == 1
		messageLength := binary.BigEndian.Uint32(frameHeader[1:5])

		// Read message body
		body = make([]byte, messageLength)
		if _, err := io.ReadFull(r.Body, body); err != nil {
			s.writeGRPCError(w, NewError(CodeInternal, "failed to read gRPC message body"))
			return
		}
		defer r.Body.Close()
	} else {
		// Non-gRPC: read entire body
		body, err = io.ReadAll(r.Body)
		if err != nil {
			s.writeError(w, r, fmt.Errorf("failed to read body: %w", err))
			return
		}
		defer r.Body.Close()

		// Check if this is a Connect protocol request with framing
		if p.isConnect && len(body) >= 5 {
			// Check if it looks like Connect framing (5-byte header)
			_ = body[0] // flags
			length := binary.BigEndian.Uint32(body[1:5])
			if int(length) == len(body)-5 {
				// This is a framed message, extract the actual message
				body = body[5:]
			}
		}
	}

	// Handle compression if needed
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
	inputVal, decodeErr := s.decodeInput(r.Header.Get("Content-Type"), body, ctx)
	if decodeErr != nil {
		if p.isGRPC {
			s.writeGRPCError(w, decodeErr.(*Error))
		} else {
			s.writeError(w, r, decodeErr)
		}
		return
	}

	// Validate if enabled
	if err := s.validateInput(inputVal, ctx); err != nil {
		if p.isGRPC {
			s.writeGRPCError(w, err.(*Error))
		} else {
			s.writeError(w, r, err)
		}
		return
	}

	// Create stream implementation
	baseStream := newServerStreamWriter(w, r, ctx, p)

	// Call the handler
	handlerValue := reflect.ValueOf(ctx.method.Handler)

	// Add handler context to the request context
	reqCtx = context.WithValue(reqCtx, handlerContextKey, ctx)

	// The handler has been wrapped to accept untyped arguments
	// Call it with the base stream directly

	// Type assert to the wrapped handler signature
	if wrappedHandler, ok := ctx.method.Handler.(func(context.Context, any, any) error); ok {
		// Call the wrapped handler
		err := wrappedHandler(reqCtx, inputVal.Interface(), baseStream)
		if err != nil {
			baseStream.sendError(err)
			return
		}
	} else {
		// Fallback to reflection
		results := handlerValue.Call([]reflect.Value{
			reflect.ValueOf(reqCtx),
			inputVal,
			reflect.ValueOf(baseStream),
		})

		if !results[0].IsNil() {
			err := results[0].Interface().(error)
			baseStream.sendError(err)
			return
		}
	}

	// Finalize the stream
	baseStream.finalize()
}

// handleClientStreamRequest handles client-streaming RPC requests
func (s *Service) handleClientStreamRequest(w http.ResponseWriter, r *http.Request, _ *handlerContext, p protocolInfo) {
	// For now, return unimplemented
	err := NewError(CodeUnimplemented, "Client streaming not yet implemented")
	if p.isConnect {
		s.writeConnectError(w, r, err)
	} else if p.isGRPC {
		s.writeGRPCError(w, err)
	} else {
		http.Error(w, err.Error(), http.StatusNotImplemented)
	}
}

// handleBidiStreamRequest handles bidirectional streaming RPC requests
func (s *Service) handleBidiStreamRequest(w http.ResponseWriter, r *http.Request, _ *handlerContext, p protocolInfo) {
	// For now, return unimplemented
	err := NewError(CodeUnimplemented, "Bidirectional streaming not yet implemented")
	if p.isConnect {
		s.writeConnectError(w, r, err)
	} else if p.isGRPC {
		s.writeGRPCError(w, err)
	} else {
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
		flushPeriod: 10 * time.Millisecond, // Flush every 10ms or after each message in low-throughput scenarios
		lastFlush:   time.Now(),
	}

	// Pre-determine encoding function based on protocol
	isJSON := p.wantsJSON
	if p.isGRPC && !isJSON {
		// gRPC protobuf encoding
		s.encodeFunc = func(msg any) ([]byte, error) {
			return ctx.outputCodec.MarshalStruct(msg)
		}
	} else if ctx.useProtoOutput && !isJSON {
		// Connect protobuf encoding
		s.encodeFunc = func(msg any) ([]byte, error) {
			if protoMsg, ok := msg.(proto.Message); ok {
				return proto.Marshal(protoMsg)
			}
			return nil, fmt.Errorf("expected proto.Message, got %T", msg)
		}
	} else if isJSON {
		// JSON encoding
		s.encodeFunc = json.Marshal
	} else {
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
	if s.protocol.isConnect {
		writeErr = s.sendConnectMessage(data)
	} else if s.protocol.isGRPC {
		writeErr = s.sendGRPCMessage(data)
	} else {
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
	frameSize := 5 + len(data)
	frameBuf := s.getFrameBuffer(frameSize)
	defer s.putFrameBuffer(frameBuf)

	// Build frame in single buffer
	frame := (*frameBuf)[:frameSize]
	frame[0] = 0                                              // flags (0 = no compression)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data))) //nolint:gosec // length is bounded by message size limits
	copy(frame[5:], data)

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
	frameSize := 5 + len(data)
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

	if s.protocol.isConnect && !s.connectEnded {
		// Send end-of-stream marker for Connect
		endMessage := []byte("{}")
		if _, err := s.w.Write([]byte{0x02}); err != nil { // End-of-stream flag
			return
		}
		if err := binary.Write(s.w, binary.BigEndian, uint32(len(endMessage))); err != nil { //nolint:gosec // bounded by message size
			return
		}
		if _, err := s.w.Write(endMessage); err != nil {
			return
		}

		// Apply trailers as headers with "trailer-" prefix
		if s.ctx.responseTrailers != nil {
			for key, values := range s.ctx.responseTrailers {
				for _, value := range values {
					s.w.Header().Add("trailer-"+key, value)
				}
			}
		}

		// Flush for Connect protocol
		if s.flusher != nil {
			s.flusher.Flush()
		}
	} else if s.protocol.isGRPC {
		// For gRPC, set trailers without flushing
		// The trailers will be sent automatically when the handler returns
		trailer := s.w.Header()
		trailer.Set("grpc-status", "0")
		trailer.Set("grpc-message", "")

		// Apply custom trailers
		if s.ctx.responseTrailers != nil {
			for key, values := range s.ctx.responseTrailers {
				for _, value := range values {
					trailer.Add(key, value)
				}
			}
		}
		// DO NOT flush for gRPC - let the HTTP/2 transport handle trailer sending
	} else if s.flusher != nil {
		// For other protocols, flush to ensure data is sent
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
