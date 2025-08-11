package rpc

import (
	"context"
	"net/http"
	"sync"
)

// bidiStream implements bidirectional streaming
type bidiStream struct {
	reader *clientStreamReader
	writer *serverStreamWriter
	mu     sync.Mutex
	closed bool
	ctx    context.Context
	cancel context.CancelFunc
}

func newBidiStream(r *http.Request, w http.ResponseWriter, ctx *handlerContext, p protocolInfo) *bidiStream {
	streamCtx, cancel := context.WithCancel(r.Context())

	return &bidiStream{
		reader: newClientStreamReader(r, ctx, p),
		writer: newServerStreamWriter(w, r, ctx, p),
		ctx:    streamCtx,
		cancel: cancel,
	}
}

// Context returns the stream context
func (b *bidiStream) Context() context.Context {
	return b.ctx
}

// Send sends a message to the client
func (b *bidiStream) Send(msg any) error {
	return b.writer.Send(msg)
}

// Recv receives a message from the client
func (b *bidiStream) Recv() (any, error) {
	return b.reader.Recv()
}

// Close closes the bidirectional stream
func (b *bidiStream) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	b.cancel()

	// Finalize writer first
	b.writer.finalize()

	// Then close reader
	return b.reader.Close()
}

// Implement typed bidirectional stream
type typedBidiStream[TIn, TOut any] struct {
	stream *bidiStream
}

func (b *typedBidiStream[TIn, TOut]) Context() context.Context {
	return b.stream.Context()
}

func (b *typedBidiStream[TIn, TOut]) Send(msg *TOut) error {
	return b.stream.Send(msg)
}

func (b *typedBidiStream[TIn, TOut]) Recv() (*TIn, error) {
	msg, err := b.stream.Recv()
	if err != nil {
		return nil, err
	}
	// Type assertion should be safe since we control the decode function
	return msg.(*TIn), nil
}
