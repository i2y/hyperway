package rpc

import (
	"fmt"
	"reflect"
)

// validateStreamingHandler validates the handler based on the stream type
func (s *Service) validateStreamingHandler(method *Method) error {
	handlerType := reflect.TypeOf(method.Handler)
	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	switch method.StreamType {
	case StreamTypeUnary:
		// Expected: func(context.Context, *Input) (*Output, error)
		if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
			return fmt.Errorf("unary handler must have signature func(context.Context, *Input) (*Output, error)")
		}
		// Extract types if not provided
		if method.InputType == nil {
			method.InputType = handlerType.In(1).Elem()
		}
		if method.OutputType == nil {
			method.OutputType = handlerType.Out(0).Elem()
		}

	case StreamTypeServerStream:
		// Expected: func(context.Context, *Input, ServerStream[Output]) error
		if handlerType.NumIn() != 3 || handlerType.NumOut() != 1 {
			return fmt.Errorf("server stream handler must have signature func(context.Context, *Input, ServerStream[Output]) error")
		}
		// For server streaming, we need to extract types differently
		// Input type is the second parameter
		if method.InputType == nil {
			method.InputType = handlerType.In(1).Elem()
		}
		// Output type needs to be set from the method builder

	case StreamTypeClientStream:
		// Expected: func(context.Context, ClientStream[Input]) (*Output, error)
		if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
			return fmt.Errorf("client stream handler must have signature func(context.Context, ClientStream[Input]) (*Output, error)")
		}
		// For client streaming, output type is in the return
		if method.OutputType == nil {
			method.OutputType = handlerType.Out(0).Elem()
		}
		// Input type needs to be set from the method builder

	case StreamTypeBidiStream:
		// Expected: func(context.Context, BidiStream[Input, Output]) error
		if handlerType.NumIn() != 2 || handlerType.NumOut() != 1 {
			return fmt.Errorf("bidi stream handler must have signature func(context.Context, BidiStream[Input, Output]) error")
		}
		// Both types need to be set from the method builder

	default:
		return fmt.Errorf("unknown stream type: %v", method.StreamType)
	}

	return nil
}

// RegisterStreamingMethod adds a streaming method to the service
func (s *Service) RegisterStreamingMethod(method *Method) error {
	// Validate method
	if method.Name == "" {
		return fmt.Errorf("method name is required")
	}
	if method.Handler == nil {
		return fmt.Errorf("method handler is required")
	}

	// Validate handler signature based on stream type
	if err := s.validateStreamingHandler(method); err != nil {
		return err
	}

	// For streaming methods, types must be provided by the builder
	if method.InputType == nil {
		return fmt.Errorf("input type is required for streaming method %s", method.Name)
	}
	if method.OutputType == nil {
		return fmt.Errorf("output type is required for streaming method %s", method.Name)
	}

	// Auto-detect protobuf types
	s.detectProtobufTypes(method)

	// Build message descriptors to ensure they're cached
	// Skip if we're using protobuf types (they have their own descriptors)
	if method.ProtoInput == nil {
		_, err := s.builder.BuildMessage(method.InputType)
		if err != nil {
			return fmt.Errorf("failed to build input descriptor for %s: %w", method.Name, err)
		}
	}

	if method.ProtoOutput == nil {
		_, err := s.builder.BuildMessage(method.OutputType)
		if err != nil {
			return fmt.Errorf("failed to build output descriptor for %s: %w", method.Name, err)
		}
	}

	// Don't wrap the handler - we'll handle it at runtime

	s.methods[method.Name] = method
	return nil
}
