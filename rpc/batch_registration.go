package rpc

import (
	"context"
	"fmt"
	"reflect"
)

// MethodDefinition represents a method that can be registered to a service.
type MethodDefinition interface {
	register(*Service) error
	getName() string
}

// unaryMethod represents a unary RPC method definition.
type unaryMethod[TIn, TOut any] struct {
	name    string
	handler func(context.Context, *TIn) (*TOut, error)
}

func (m *unaryMethod[TIn, TOut]) register(s *Service) error {
	return RegisterAs(s, m.name, m.handler)
}

func (m *unaryMethod[TIn, TOut]) getName() string {
	return m.name
}

// serverStreamMethod represents a server-streaming RPC method definition.
type serverStreamMethod[TIn, TOut any] struct {
	name    string
	handler func(context.Context, *TIn, ServerStream[TOut]) error
}

func (m *serverStreamMethod[TIn, TOut]) register(s *Service) error {
	return RegisterServerStreamAs(s, m.name, m.handler)
}

func (m *serverStreamMethod[TIn, TOut]) getName() string {
	return m.name
}

// clientStreamMethod represents a client-streaming RPC method definition.
type clientStreamMethod[TIn, TOut any] struct {
	name    string
	handler func(context.Context, ClientStream[TIn]) (*TOut, error)
}

func (m *clientStreamMethod[TIn, TOut]) register(s *Service) error {
	return RegisterClientStreamAs(s, m.name, m.handler)
}

func (m *clientStreamMethod[TIn, TOut]) getName() string {
	return m.name
}

// bidiStreamMethod represents a bidirectional streaming RPC method definition.
type bidiStreamMethod[TIn, TOut any] struct {
	name    string
	handler func(context.Context, BidiStream[TIn, TOut]) error
}

func (m *bidiStreamMethod[TIn, TOut]) register(s *Service) error {
	return RegisterBidiStreamAs(s, m.name, m.handler)
}

func (m *bidiStreamMethod[TIn, TOut]) getName() string {
	return m.name
}

// Unary creates a unary RPC method definition.
func Unary[TIn, TOut any](name string, handler func(context.Context, *TIn) (*TOut, error)) MethodDefinition {
	return &unaryMethod[TIn, TOut]{
		name:    name,
		handler: handler,
	}
}

// ServerStreamDef creates a server-streaming RPC method definition.
func ServerStreamDef[TIn, TOut any](name string, handler func(context.Context, *TIn, ServerStream[TOut]) error) MethodDefinition {
	return &serverStreamMethod[TIn, TOut]{
		name:    name,
		handler: handler,
	}
}

// ClientStreamDef creates a client-streaming RPC method definition.
func ClientStreamDef[TIn, TOut any](name string, handler func(context.Context, ClientStream[TIn]) (*TOut, error)) MethodDefinition {
	return &clientStreamMethod[TIn, TOut]{
		name:    name,
		handler: handler,
	}
}

// BidiStreamDef creates a bidirectional streaming RPC method definition.
func BidiStreamDef[TIn, TOut any](name string, handler func(context.Context, BidiStream[TIn, TOut]) error) MethodDefinition {
	return &bidiStreamMethod[TIn, TOut]{
		name:    name,
		handler: handler,
	}
}

// RegisterAll registers multiple methods to the service.
func (s *Service) RegisterAll(methods ...MethodDefinition) error {
	// First validate all methods have unique names
	seen := make(map[string]bool)
	for _, method := range methods {
		name := method.getName()
		if seen[name] {
			return fmt.Errorf("duplicate method name: %s", name)
		}
		seen[name] = true
	}

	// Register all methods
	for _, method := range methods {
		if err := method.register(s); err != nil {
			return fmt.Errorf("failed to register method %s: %w", method.getName(), err)
		}
	}

	return nil
}

// MustRegisterAll registers multiple methods to the service and panics on error.
func (s *Service) MustRegisterAll(methods ...MethodDefinition) {
	if err := s.RegisterAll(methods...); err != nil {
		panic(err)
	}
}

// MethodGroup provides a way to register multiple methods with shared types.
type MethodGroup[TContext any] struct {
	service *Service
	methods []MethodDefinition
}

// Group creates a new method group for registering methods with shared context type.
func (s *Service) Group() *MethodGroup[any] {
	return &MethodGroup[any]{
		service: s,
		methods: []MethodDefinition{},
	}
}

// TypedGroup creates a new method group with a specific context type.
func TypedGroup[TContext any](s *Service) *MethodGroup[TContext] {
	return &MethodGroup[TContext]{
		service: s,
		methods: []MethodDefinition{},
	}
}

// Add adds a method to the group.
func (g *MethodGroup[TContext]) Add(method MethodDefinition) *MethodGroup[TContext] {
	g.methods = append(g.methods, method)
	return g
}

// Register registers all methods in the group.
func (g *MethodGroup[TContext]) Register() error {
	return g.service.RegisterAll(g.methods...)
}

// MustRegister registers all methods in the group and panics on error.
func (g *MethodGroup[TContext]) MustRegister() {
	g.service.MustRegisterAll(g.methods...)
}

// ServiceRegistrar provides an interface for services that can self-register.
type ServiceRegistrar interface {
	RegisterMethods(*Service) error
}

// RegisterService registers all methods from a ServiceRegistrar.
func (s *Service) RegisterService(registrar ServiceRegistrar) error {
	return registrar.RegisterMethods(s)
}

// MustRegisterService registers all methods from a ServiceRegistrar and panics on error.
func (s *Service) MustRegisterService(registrar ServiceRegistrar) {
	if err := s.RegisterService(registrar); err != nil {
		panic(err)
	}
}

// AutoRegister attempts to automatically register methods from a struct.
// This is experimental and uses reflection to find methods with appropriate signatures.
func (s *Service) AutoRegister(service any) error {
	serviceType := reflect.TypeOf(service)

	if serviceType.Kind() == reflect.Ptr {
		serviceType = serviceType.Elem()
	}

	if serviceType.Kind() != reflect.Struct {
		return fmt.Errorf("service must be a struct or pointer to struct")
	}

	registered := 0

	// Look for methods on the service
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		if !method.IsExported() {
			continue
		}

		// Check if the method has a handler-like signature
		methodType := method.Type
		if methodType.NumIn() < 2 || methodType.NumOut() < 1 {
			continue
		}

		// Skip the receiver
		if methodType.In(0) != serviceType {
			continue
		}

		// Check for context.Context as first parameter
		if methodType.In(1).String() != "context.Context" {
			continue
		}

		// This looks like a potential handler method
		// For now, we'll skip auto-registration due to complexity
		// but this shows how it could be done
		registered++
	}

	if registered == 0 {
		return fmt.Errorf("no methods found to register")
	}

	return fmt.Errorf("auto-registration is not yet fully implemented")
}
