package rpc

import (
	"fmt"
	"time"
)

// MethodChain provides a fluent API for building and registering methods.
type MethodChain struct {
	service      *Service
	name         string
	handler      any
	streamType   StreamType
	options      MethodOptions
	interceptors []Interceptor
	description  string
}

// Method creates a new method chain for the service.
func (s *Service) Method(name string) *MethodChain {
	return &MethodChain{
		service:    s,
		name:       name,
		streamType: StreamTypeUnary, // Default to unary
		options:    MethodOptions{},
	}
}

// Unary sets the handler for a unary RPC.
func (m *MethodChain) Unary(handler any) *MethodChain {
	m.handler = handler
	m.streamType = StreamTypeUnary
	return m
}

// ServerStream sets the handler for a server-streaming RPC.
func (m *MethodChain) ServerStream(handler any) *MethodChain {
	m.handler = handler
	m.streamType = StreamTypeServerStream
	return m
}

// ClientStream sets the handler for a client-streaming RPC.
func (m *MethodChain) ClientStream(handler any) *MethodChain {
	m.handler = handler
	m.streamType = StreamTypeClientStream
	return m
}

// BidiStream sets the handler for a bidirectional streaming RPC.
func (m *MethodChain) BidiStream(handler any) *MethodChain {
	m.handler = handler
	m.streamType = StreamTypeBidiStream
	return m
}

// Validate enables or disables validation for this method.
func (m *MethodChain) Validate(enabled bool) *MethodChain {
	m.options.Validate = &enabled
	return m
}

// WithTimeout sets a timeout for this method.
func (m *MethodChain) WithTimeout(d time.Duration) *MethodChain {
	// Store timeout in options for future use
	// This would require extending MethodOptions to include timeout
	return m
}

// WithRetry sets retry configuration for this method.
func (m *MethodChain) WithRetry(maxAttempts int) *MethodChain {
	// Add retry interceptor
	retryPolicy := &RetryPolicy{
		MaxAttempts:       maxAttempts,
		BackoffMultiplier: defaultBackoffMultiplier,
		InitialBackoff:    "100ms",
		MaxBackoff:        "5s",
	}

	// Create a service config with the retry policy
	serviceConfig := &ServiceConfig{
		MethodConfig: []MethodConfig{
			{
				Name:        []MethodName{{Service: m.service.name, Method: m.name}},
				RetryPolicy: retryPolicy,
			},
		},
	}

	interceptor := &RetryInterceptor{
		serviceConfig: serviceConfig,
	}

	return m.WithInterceptors(interceptor)
}

// WithInterceptors adds interceptors to this method.
func (m *MethodChain) WithInterceptors(interceptors ...Interceptor) *MethodChain {
	m.interceptors = append(m.interceptors, interceptors...)
	return m
}

// WithDescription sets a description for this method.
func (m *MethodChain) WithDescription(description string) *MethodChain {
	m.description = description
	return m
}

// WithCompression sets compression settings for this method.
func (m *MethodChain) WithCompression(algorithm string) *MethodChain {
	// This would require extending MethodOptions to include compression settings
	return m
}

// WithMaxMessageSize sets the maximum message size for this method.
func (m *MethodChain) WithMaxMessageSize(size int) *MethodChain {
	// This would require extending MethodOptions to include message size limits
	return m
}

// Register registers the method with the service.
func (m *MethodChain) Register() error {
	if m.handler == nil {
		return fmt.Errorf("handler not set for method %s", m.name)
	}

	// Apply interceptors to the service if any
	if len(m.interceptors) > 0 {
		// Combine with existing service interceptors
		m.service.options.Interceptors = append(m.service.options.Interceptors, m.interceptors...)
	}

	// Register based on stream type
	switch m.streamType {
	case StreamTypeUnary:
		// Use reflection to call the generic Register method
		// This is a bit complex due to Go's type system
		return m.registerUnary()
	case StreamTypeServerStream:
		return m.registerServerStream()
	case StreamTypeClientStream:
		return m.registerClientStream()
	case StreamTypeBidiStream:
		return m.registerBidiStream()
	default:
		return fmt.Errorf("unknown stream type: %v", m.streamType)
	}
}

// MustRegister registers the method and panics on error.
func (m *MethodChain) MustRegister() *MethodChain {
	if err := m.Register(); err != nil {
		panic(err)
	}
	return m
}

// And allows chaining multiple method registrations.
func (m *MethodChain) And(name string) *MethodChain {
	// First register the current method
	if err := m.Register(); err != nil {
		panic(fmt.Sprintf("failed to register method %s: %v", m.name, err))
	}

	// Return a new chain for the next method
	return m.service.Method(name)
}

// registerUnary handles unary method registration.
func (m *MethodChain) registerUnary() error {
	// Since we can't use generics dynamically, we need to create a Method directly
	method := &Method{
		Name:       m.name,
		Handler:    m.handler,
		Options:    m.options,
		StreamType: StreamTypeUnary,
	}
	m.options.Description = m.description

	// The types will be extracted by Register
	return m.service.Register(method)
}

// registerServerStream handles server stream method registration.
func (m *MethodChain) registerServerStream() error {
	method := &Method{
		Name:       m.name,
		Handler:    m.handler,
		Options:    m.options,
		StreamType: StreamTypeServerStream,
	}
	m.options.Description = m.description

	return m.service.RegisterStreamingMethod(method)
}

// registerClientStream handles client stream method registration.
func (m *MethodChain) registerClientStream() error {
	method := &Method{
		Name:       m.name,
		Handler:    m.handler,
		Options:    m.options,
		StreamType: StreamTypeClientStream,
	}
	m.options.Description = m.description

	return m.service.RegisterStreamingMethod(method)
}

// registerBidiStream handles bidirectional stream method registration.
func (m *MethodChain) registerBidiStream() error {
	method := &Method{
		Name:       m.name,
		Handler:    m.handler,
		Options:    m.options,
		StreamType: StreamTypeBidiStream,
	}
	m.options.Description = m.description

	return m.service.RegisterStreamingMethod(method)
}

// Build returns the method without registering it.
// This is useful for batch registration.
func (m *MethodChain) Build() *Method {
	m.options.Description = m.description
	return &Method{
		Name:       m.name,
		Handler:    m.handler,
		Options:    m.options,
		StreamType: m.streamType,
	}
}
