package rpc

import (
	"context"
	"fmt"
	"reflect"
)

// RegisterClientStream registers a client-streaming RPC method.
// The handler must have signature: func(context.Context, ClientStream[Input]) (*Output, error)
func RegisterClientStream[TIn, TOut any](s *Service, name string, handler ClientStreamHandler[TIn, TOut]) error {
	// Wrap the handler to work with the internal stream type
	wrappedHandler := func(ctx context.Context, stream any) (any, error) {
		// Create typed stream wrapper
		typedStream := &typedClientStream[TIn]{
			reader: stream.(*clientStreamReader),
		}

		return handler(ctx, typedStream)
	}

	method := &Method{
		Name:       name,
		Handler:    wrappedHandler,
		InputType:  reflect.TypeOf((*TIn)(nil)).Elem(),
		OutputType: reflect.TypeOf((*TOut)(nil)).Elem(),
		StreamType: StreamTypeClientStream,
	}

	return s.RegisterStreamingMethod(method)
}

// MustRegisterClientStream is like RegisterClientStream but panics on error.
func MustRegisterClientStream[TIn, TOut any](s *Service, name string, handler ClientStreamHandler[TIn, TOut]) {
	if err := RegisterClientStream(s, name, handler); err != nil {
		panic(fmt.Sprintf("failed to register client stream %s: %v", name, err))
	}
}

// RegisterBidiStream registers a bidirectional streaming RPC method.
// The handler must have signature: func(context.Context, BidiStream[Input, Output]) error
func RegisterBidiStream[TIn, TOut any](s *Service, name string, handler BidiStreamHandler[TIn, TOut]) error {
	// Wrap the handler to work with the internal stream type
	wrappedHandler := func(ctx context.Context, stream any) error {
		// Create typed stream wrapper
		typedStream := &typedBidiStream[TIn, TOut]{
			stream: stream.(*bidiStream),
		}

		return handler(ctx, typedStream)
	}

	method := &Method{
		Name:       name,
		Handler:    wrappedHandler,
		InputType:  reflect.TypeOf((*TIn)(nil)).Elem(),
		OutputType: reflect.TypeOf((*TOut)(nil)).Elem(),
		StreamType: StreamTypeBidiStream,
	}

	return s.RegisterStreamingMethod(method)
}

// MustRegisterBidiStream is like RegisterBidiStream but panics on error.
func MustRegisterBidiStream[TIn, TOut any](s *Service, name string, handler BidiStreamHandler[TIn, TOut]) {
	if err := RegisterBidiStream(s, name, handler); err != nil {
		panic(fmt.Sprintf("failed to register bidi stream %s: %v", name, err))
	}
}
