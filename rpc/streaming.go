// Package rpc provides streaming RPC support.
package rpc

import (
	"context"
	"io"
	"reflect"
)

// StreamType defines the type of streaming RPC.
type StreamType int

const (
	// StreamTypeUnary is a unary RPC (no streaming).
	StreamTypeUnary StreamType = iota
	// StreamTypeServerStream is a server-streaming RPC.
	StreamTypeServerStream
	// StreamTypeClientStream is a client-streaming RPC.
	StreamTypeClientStream
	// StreamTypeBidiStream is a bidirectional streaming RPC.
	StreamTypeBidiStream
)

// ServerStream represents a server-side stream.
type ServerStream[T any] interface {
	// Send sends a message to the client.
	Send(*T) error
	// Context returns the context for this stream.
	Context() context.Context
}

// ClientStream represents a client-side stream.
type ClientStream[T any] interface {
	// Recv receives a message from the client.
	Recv() (*T, error)
	// Context returns the context for this stream.
	Context() context.Context
}

// BidiStream represents a bidirectional stream.
type BidiStream[TIn, TOut any] interface {
	// Send sends a message to the client.
	Send(*TOut) error
	// Recv receives a message from the client.
	Recv() (*TIn, error)
	// Context returns the context for this stream.
	Context() context.Context
}

// StreamingHandlers define different handler types for streaming RPCs.

// ServerStreamHandler handles server-streaming RPCs.
type ServerStreamHandler[TIn, TOut any] func(context.Context, *TIn, ServerStream[TOut]) error

// ClientStreamHandler handles client-streaming RPCs.
type ClientStreamHandler[TIn, TOut any] func(context.Context, ClientStream[TIn]) (*TOut, error)

// BidiStreamHandler handles bidirectional streaming RPCs.
type BidiStreamHandler[TIn, TOut any] func(context.Context, BidiStream[TIn, TOut]) error

// streamImpl implements the streaming interfaces.
//
//nolint:unused // Will be used when streaming is fully implemented
type streamImpl struct {
	ctx      context.Context
	send     chan any
	recv     chan any
	sendErr  chan error
	recvErr  chan error
	closed   bool
	sendType reflect.Type
	recvType reflect.Type
}

// newStream creates a new stream implementation.
//
//nolint:unused // Will be used when streaming is fully implemented
func newStream(ctx context.Context, sendType, recvType reflect.Type) *streamImpl {
	return &streamImpl{
		ctx:      ctx,
		send:     make(chan any, 1),
		recv:     make(chan any, 1),
		sendErr:  make(chan error, 1),
		recvErr:  make(chan error, 1),
		sendType: sendType,
		recvType: recvType,
	}
}

// Context returns the stream context.
//
//nolint:unused // Will be used when streaming is fully implemented
func (s *streamImpl) Context() context.Context {
	return s.ctx
}

// Send sends a message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (s *streamImpl) Send(msg any) error {
	if s.closed {
		return io.EOF
	}

	select {
	case s.send <- msg:
		return nil
	case err := <-s.sendErr:
		return err
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// Recv receives a message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (s *streamImpl) Recv() (any, error) {
	if s.closed {
		return nil, io.EOF
	}

	select {
	case msg := <-s.recv:
		return msg, nil
	case err := <-s.recvErr:
		return nil, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

// Close closes the stream.
//
//nolint:unused // Will be used when streaming is fully implemented
func (s *streamImpl) Close() {
	s.closed = true
	close(s.send)
	close(s.recv)
	close(s.sendErr)
	close(s.recvErr)
}

// serverStreamImpl implements ServerStream.
//
//nolint:unused // Will be used when streaming is fully implemented
type serverStreamImpl[T any] struct {
	*streamImpl
}

// Send sends a typed message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (s *serverStreamImpl[T]) Send(msg *T) error {
	return s.streamImpl.Send(msg)
}

// clientStreamImpl implements ClientStream.
//
//nolint:unused // Will be used when streaming is fully implemented
type clientStreamImpl[T any] struct {
	*streamImpl
}

// Recv receives a typed message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (c *clientStreamImpl[T]) Recv() (*T, error) {
	msg, err := c.streamImpl.Recv()
	if err != nil {
		return nil, err
	}
	return msg.(*T), nil
}

// bidiStreamImpl implements BidiStream.
//
//nolint:unused // Will be used when streaming is fully implemented
type bidiStreamImpl[TIn, TOut any] struct {
	*streamImpl
}

// Send sends a typed message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (b *bidiStreamImpl[TIn, TOut]) Send(msg *TOut) error {
	return b.streamImpl.Send(msg)
}

// Recv receives a typed message.
//
//nolint:unused // Will be used when streaming is fully implemented
func (b *bidiStreamImpl[TIn, TOut]) Recv() (*TIn, error) {
	msg, err := b.streamImpl.Recv()
	if err != nil {
		return nil, err
	}
	return msg.(*TIn), nil
}
