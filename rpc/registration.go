package rpc

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unicode"
)

// Handler represents a typed RPC handler function.
type Handler[TIn, TOut any] func(context.Context, *TIn) (*TOut, error)

// getFunctionName extracts the function name from a handler.
func getFunctionName(fn any) string {
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		return ""
	}

	ptr := fnValue.Pointer()
	f := runtime.FuncForPC(ptr)
	if f == nil {
		return ""
	}

	fullName := f.Name()

	// Handle different function name patterns
	// "main.createUser" -> "createUser"
	// "main.(*UserService).CreateUser" -> "CreateUser"
	// "main.UserService.CreateUser-fm" -> "CreateUser"

	// Remove package prefix
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot != -1 {
		fullName = fullName[lastDot+1:]
	}

	// Remove method receiver if present
	if strings.Contains(fullName, ").") {
		parts := strings.Split(fullName, ").")
		if len(parts) > 1 {
			fullName = parts[1]
		}
	}

	// Remove "-fm" suffix (method value)
	fullName = strings.TrimSuffix(fullName, "-fm")

	// Handle closures and anonymous functions
	if strings.Contains(fullName, "func") || strings.Contains(fullName, "glob") {
		return ""
	}

	return fullName
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Register registers a unary RPC method with automatic name generation.
func Register[TIn, TOut any](svc *Service, handler Handler[TIn, TOut]) error {
	name := getFunctionName(handler)
	if name == "" {
		return fmt.Errorf("cannot determine function name (anonymous functions are not supported)")
	}
	// Capitalize first letter for RPC convention
	name = capitalizeFirst(name)
	return RegisterAs(svc, name, handler)
}

// RegisterAs registers a unary RPC method with an explicit name.
func RegisterAs[TIn, TOut any](svc *Service, name string, handler Handler[TIn, TOut]) error {
	method := &Method{
		Name:       name,
		Handler:    handler,
		InputType:  reflect.TypeOf((*TIn)(nil)).Elem(),
		OutputType: reflect.TypeOf((*TOut)(nil)).Elem(),
		StreamType: StreamTypeUnary,
	}
	return svc.Register(method)
}

// MustRegister registers a unary RPC method with automatic name generation and panics on error.
func MustRegister[TIn, TOut any](svc *Service, handler Handler[TIn, TOut]) {
	if err := Register(svc, handler); err != nil {
		panic(err)
	}
}

// MustRegisterAs registers a unary RPC method with an explicit name and panics on error.
func MustRegisterAs[TIn, TOut any](svc *Service, name string, handler Handler[TIn, TOut]) {
	if err := RegisterAs(svc, name, handler); err != nil {
		panic(err)
	}
}

// RegisterServerStream registers a server-streaming RPC method with automatic name generation.
func RegisterServerStream[TIn, TOut any](svc *Service, handler ServerStreamHandler[TIn, TOut]) error {
	name := getFunctionName(handler)
	if name == "" {
		return fmt.Errorf("cannot determine function name (anonymous functions are not supported)")
	}
	// Capitalize first letter for RPC convention
	name = capitalizeFirst(name)
	return RegisterServerStreamAs(svc, name, handler)
}

// RegisterServerStreamAs registers a server-streaming RPC method with an explicit name.
func RegisterServerStreamAs[TIn, TOut any](svc *Service, name string, handler ServerStreamHandler[TIn, TOut]) error {
	// Wrap the handler to work with the internal stream type
	wrappedHandler := func(ctx context.Context, req any, stream any) error {
		// Type assert the request
		typedReq, ok := req.(*TIn)
		if !ok {
			return fmt.Errorf("invalid request type: expected *%T, got %T", (*TIn)(nil), req)
		}

		// Type assert the stream
		typedStream, ok := stream.(ServerStream[TOut])
		if !ok {
			// If direct cast fails, wrap the stream
			baseStream, ok := stream.(*serverStreamWriter)
			if !ok {
				return fmt.Errorf("invalid stream type: %T", stream)
			}
			typedStream = &typedServerStream[TOut]{baseStream}
		}

		// Call the original handler
		return handler(ctx, typedReq, typedStream)
	}

	method := &Method{
		Name:       name,
		Handler:    wrappedHandler,
		InputType:  reflect.TypeOf((*TIn)(nil)).Elem(),
		OutputType: reflect.TypeOf((*TOut)(nil)).Elem(),
		StreamType: StreamTypeServerStream,
	}

	return svc.RegisterStreamingMethod(method)
}

// MustRegisterServerStream registers a server-streaming RPC method with automatic name generation and panics on error.
func MustRegisterServerStream[TIn, TOut any](svc *Service, handler ServerStreamHandler[TIn, TOut]) {
	if err := RegisterServerStream(svc, handler); err != nil {
		panic(err)
	}
}

// MustRegisterServerStreamAs registers a server-streaming RPC method with an explicit name and panics on error.
func MustRegisterServerStreamAs[TIn, TOut any](svc *Service, name string, handler ServerStreamHandler[TIn, TOut]) {
	if err := RegisterServerStreamAs(svc, name, handler); err != nil {
		panic(err)
	}
}

// RegisterClientStream registers a client-streaming RPC method with automatic name generation.
func RegisterClientStream[TIn, TOut any](svc *Service, handler ClientStreamHandler[TIn, TOut]) error {
	name := getFunctionName(handler)
	if name == "" {
		return fmt.Errorf("cannot determine function name (anonymous functions are not supported)")
	}
	// Capitalize first letter for RPC convention
	name = capitalizeFirst(name)
	return RegisterClientStreamAs(svc, name, handler)
}

// RegisterClientStreamAs registers a client-streaming RPC method with an explicit name.
func RegisterClientStreamAs[TIn, TOut any](svc *Service, name string, handler ClientStreamHandler[TIn, TOut]) error {
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

	return svc.RegisterStreamingMethod(method)
}

// MustRegisterClientStream registers a client-streaming RPC method with automatic name generation and panics on error.
func MustRegisterClientStream[TIn, TOut any](svc *Service, handler ClientStreamHandler[TIn, TOut]) {
	if err := RegisterClientStream(svc, handler); err != nil {
		panic(err)
	}
}

// MustRegisterClientStreamAs registers a client-streaming RPC method with an explicit name and panics on error.
func MustRegisterClientStreamAs[TIn, TOut any](svc *Service, name string, handler ClientStreamHandler[TIn, TOut]) {
	if err := RegisterClientStreamAs(svc, name, handler); err != nil {
		panic(err)
	}
}

// RegisterBidiStream registers a bidirectional streaming RPC method with automatic name generation.
func RegisterBidiStream[TIn, TOut any](svc *Service, handler BidiStreamHandler[TIn, TOut]) error {
	name := getFunctionName(handler)
	if name == "" {
		return fmt.Errorf("cannot determine function name (anonymous functions are not supported)")
	}
	// Capitalize first letter for RPC convention
	name = capitalizeFirst(name)
	return RegisterBidiStreamAs(svc, name, handler)
}

// RegisterBidiStreamAs registers a bidirectional streaming RPC method with an explicit name.
func RegisterBidiStreamAs[TIn, TOut any](svc *Service, name string, handler BidiStreamHandler[TIn, TOut]) error {
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

	return svc.RegisterStreamingMethod(method)
}

// MustRegisterBidiStream registers a bidirectional streaming RPC method with automatic name generation and panics on error.
func MustRegisterBidiStream[TIn, TOut any](svc *Service, handler BidiStreamHandler[TIn, TOut]) {
	if err := RegisterBidiStream(svc, handler); err != nil {
		panic(err)
	}
}

// MustRegisterBidiStreamAs registers a bidirectional streaming RPC method with an explicit name and panics on error.
func MustRegisterBidiStreamAs[TIn, TOut any](svc *Service, name string, handler BidiStreamHandler[TIn, TOut]) {
	if err := RegisterBidiStreamAs(svc, name, handler); err != nil {
		panic(err)
	}
}
