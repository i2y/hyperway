// Package rpc provides the main public API for the hyperway RPC library.
package rpc

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/gateway"
	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/schema"
)

// Interceptor is our own interceptor interface that works with dynamic types.
type Interceptor interface {
	// Intercept wraps the handler call.
	Intercept(ctx context.Context, method string, req any, handler func(context.Context, any) (any, error)) (any, error)
}

// Service represents an RPC service.
type Service struct {
	name            string
	packageName     string
	methods         map[string]*Method
	options         ServiceOptions
	builder         *schema.Builder
	validator       *validator.Validate
	handlerCtxCache map[string]*handlerContext // Cache prepared handler contexts
	serviceConfig   *ServiceConfig             // gRPC service configuration
}

// ServiceOptions configures a service.
type ServiceOptions struct {
	// Package sets the protobuf package name
	Package string
	// EnableValidation enables input validation by default
	EnableValidation bool
	// EnableReflection enables gRPC reflection
	EnableReflection bool
	// Interceptors to apply to all methods
	Interceptors []Interceptor
	// Edition sets the Protobuf edition (e.g., "2023", "2024")
	Edition string
	// UseEditions enables Protobuf Editions mode instead of proto3
	UseEditions bool
	// ServiceConfig is the gRPC service configuration (JSON string)
	ServiceConfig string
	// Description is the service-level documentation
	Description string
}

// Method represents an RPC method.
type Method struct {
	Name       string
	Handler    any
	InputType  reflect.Type
	OutputType reflect.Type
	Options    MethodOptions
}

// MethodOptions configures a method.
type MethodOptions struct {
	// Validate enables input validation for this method
	Validate *bool
	// Interceptors specific to this method
	Interceptors []Interceptor
	// Description is the method-level documentation
	Description string
}

// Global instances for performance - thread-safe and can be reused
var (
	globalValidator = validator.New()
	// Global schema builder cache - significantly speeds up service registration
	globalBuilderCache = sync.Map{} // map[packageName]*schema.Builder
)

// NewService creates a new RPC service.
func NewService(name string, opts ...ServiceOption) *Service {
	svc := &Service{
		name:            name,
		methods:         make(map[string]*Method),
		options:         ServiceOptions{},
		validator:       globalValidator, // Reuse global validator
		handlerCtxCache: make(map[string]*handlerContext),
	}

	// Apply options
	for _, opt := range opts {
		opt(&svc.options)
	}

	// Set package name from options or default to service name
	if svc.options.Package != "" {
		svc.packageName = svc.options.Package
	} else {
		svc.packageName = name
	}

	// Parse service config if provided
	if svc.options.ServiceConfig != "" {
		config, err := ParseServiceConfig(svc.options.ServiceConfig)
		if err != nil {
			// Log error but don't fail service creation
			// This matches gRPC behavior - invalid service config is ignored
			fmt.Printf("Warning: failed to parse service config: %v\n", err)
		} else {
			svc.serviceConfig = config
		}
	}

	// Get or create schema builder from global cache
	// Include edition settings in cache key to ensure different builders for different editions
	cacheKey := svc.packageName
	if svc.options.UseEditions {
		cacheKey = fmt.Sprintf("%s_editions_%s", svc.packageName, svc.options.Edition)
	}

	if cachedBuilder, ok := globalBuilderCache.Load(cacheKey); ok {
		svc.builder = cachedBuilder.(*schema.Builder)
	} else {
		builderOpts := schema.BuilderOptions{
			PackageName: svc.packageName,
		}

		// Configure editions mode if enabled
		if svc.options.UseEditions {
			builderOpts.SyntaxMode = schema.SyntaxEditions
			builderOpts.Edition = svc.options.Edition
			if builderOpts.Edition == "" {
				builderOpts.Edition = schema.Edition2023 // Default to 2023
			}
		}

		newBuilder := schema.NewBuilder(builderOpts)
		globalBuilderCache.Store(cacheKey, newBuilder)
		svc.builder = newBuilder
	}

	return svc
}

// Register adds a method to the service.
func (s *Service) Register(method *Method) error {
	// Validate method
	if method.Name == "" {
		return fmt.Errorf("method name is required")
	}
	if method.Handler == nil {
		return fmt.Errorf("method handler is required")
	}

	// Validate handler signature
	handlerType := reflect.TypeOf(method.Handler)
	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	// Expected: func(context.Context, *Input) (*Output, error)
	if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
		return fmt.Errorf("handler must have signature func(context.Context, *Input) (*Output, error)")
	}

	// Extract types if not provided
	if method.InputType == nil {
		method.InputType = handlerType.In(1).Elem()
	}
	if method.OutputType == nil {
		method.OutputType = handlerType.Out(0).Elem()
	}

	// Build message descriptors to ensure they're cached
	// This populates the builder's cache for use in gateway creation
	_, err := s.builder.BuildMessage(method.InputType)
	if err != nil {
		return fmt.Errorf("failed to build input descriptor for %s: %w", method.Name, err)
	}

	_, err = s.builder.BuildMessage(method.OutputType)
	if err != nil {
		return fmt.Errorf("failed to build output descriptor for %s: %w", method.Name, err)
	}

	s.methods[method.Name] = method
	return nil
}

// MustRegister is like Register but panics on error.
func (s *Service) MustRegister(method *Method) {
	if err := s.Register(method); err != nil {
		panic(err)
	}
}

// Handler represents a typed RPC handler function.
type Handler[TIn, TOut any] func(context.Context, *TIn) (*TOut, error)

// NewMethod creates a new method.
func NewMethod[TIn, TOut any](name string, handler Handler[TIn, TOut]) *MethodBuilder {
	// Get the input and output types from the generic parameters
	var in TIn
	var out TOut
	inputType := reflect.TypeOf(in)
	outputType := reflect.TypeOf(out)

	return &MethodBuilder{
		method: &Method{
			Name:       name,
			Handler:    handler,
			InputType:  inputType,
			OutputType: outputType,
			Options:    MethodOptions{},
		},
	}
}

// MethodBuilder provides a fluent API for building methods.
type MethodBuilder struct {
	method *Method
}

// In sets the input type.
func (m *MethodBuilder) In(example any) *MethodBuilder {
	m.method.InputType = reflect.TypeOf(example)
	if m.method.InputType.Kind() == reflect.Ptr {
		m.method.InputType = m.method.InputType.Elem()
	}
	return m
}

// Out sets the output type.
func (m *MethodBuilder) Out(example any) *MethodBuilder {
	m.method.OutputType = reflect.TypeOf(example)
	if m.method.OutputType.Kind() == reflect.Ptr {
		m.method.OutputType = m.method.OutputType.Elem()
	}
	return m
}

// Validate sets whether to validate input.
func (m *MethodBuilder) Validate(enabled bool) *MethodBuilder {
	m.method.Options.Validate = &enabled
	return m
}

// WithInterceptors adds interceptors to the method.
func (m *MethodBuilder) WithInterceptors(interceptors ...Interceptor) *MethodBuilder {
	m.method.Options.Interceptors = append(m.method.Options.Interceptors, interceptors...)
	return m
}

// WithDescription sets the method description for documentation.
func (m *MethodBuilder) WithDescription(description string) *MethodBuilder {
	m.method.Options.Description = description
	return m
}

// Build returns the built method.
func (m *MethodBuilder) Build() *Method {
	return m.method
}

// ServiceOption configures a service.
type ServiceOption func(*ServiceOptions)

// WithPackage sets the protobuf package name.
func WithPackage(pkg string) ServiceOption {
	return func(o *ServiceOptions) {
		o.Package = pkg
	}
}

// WithValidation enables validation by default.
func WithValidation(enabled bool) ServiceOption {
	return func(o *ServiceOptions) {
		o.EnableValidation = enabled
	}
}

// WithReflection enables gRPC reflection.
func WithReflection(enabled bool) ServiceOption {
	return func(o *ServiceOptions) {
		o.EnableReflection = enabled
	}
}

// ExportProto exports the service definition as a .proto file.
func (s *Service) ExportProto() (string, error) {
	// Build the complete FileDescriptorSet including service definition
	fdset := s.buildCompleteFileDescriptorSet()
	if fdset == nil || len(fdset.File) == 0 {
		return "", fmt.Errorf("no proto files to export")
	}

	// Use the proto exporter
	exporter := proto.NewExporter(proto.DefaultExportOptions())

	// Export all files
	files, err := exporter.ExportFileDescriptorSet(fdset)
	if err != nil {
		return "", fmt.Errorf("failed to export proto: %w", err)
	}

	// Find and return the service proto file
	serviceFileName := fmt.Sprintf("%s.proto", s.packageName)
	for filename, content := range files {
		if strings.HasSuffix(filename, serviceFileName) {
			return content, nil
		}
	}

	// If no service file found, return the first file
	for _, content := range files {
		return content, nil
	}

	return "", fmt.Errorf("no proto content generated")
}

// ExportAllProtos exports all proto files including dependencies.
func (s *Service) ExportAllProtos() (map[string]string, error) {
	// Build the complete FileDescriptorSet including service definition
	fdset := s.buildCompleteFileDescriptorSet()
	if fdset == nil || len(fdset.File) == 0 {
		return nil, fmt.Errorf("no proto files to export")
	}

	// Use the proto exporter
	exporter := proto.NewExporter(proto.DefaultExportOptions())

	return exporter.ExportFileDescriptorSet(fdset)
}

// GetFileDescriptorSet returns the FileDescriptorSet for this service.
func (s *Service) GetFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	return s.buildCompleteFileDescriptorSet()
}

// buildCompleteFileDescriptorSet builds a complete FileDescriptorSet including service definition.
//
//nolint:gocyclo // This function orchestrates the complete file descriptor set generation
func (s *Service) buildCompleteFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	// Create a single file that contains all message types and the service
	// This avoids duplicate type definitions across multiple files

	// Create SourceCodeInfo builder for service file
	sourceCodeInfo := schema.NewSourceCodeInfoBuilder()

	// Collect all unique message types used by this service
	messageTypes := make(map[string]reflect.Type)
	for _, method := range s.methods {
		// Add input and output types
		messageTypes[method.InputType.Name()] = method.InputType
		messageTypes[method.OutputType.Name()] = method.OutputType

		// Also collect nested types by traversing the type structure
		collectNestedTypes(method.InputType, messageTypes, s.packageName)
		collectNestedTypes(method.OutputType, messageTypes, s.packageName)
	}

	// Create a new builder for this specific file to avoid conflicts
	builderOpts := schema.BuilderOptions{
		PackageName: s.packageName,
		SyntaxMode:  s.builder.GetSyntaxMode(),
		Edition:     s.builder.GetEdition(),
	}

	// Configure editions mode if enabled
	if s.options.UseEditions {
		builderOpts.SyntaxMode = schema.SyntaxEditions
		builderOpts.Edition = s.options.Edition
		if builderOpts.Edition == "" {
			builderOpts.Edition = schema.Edition2023
		}
	}

	fileBuilder := schema.NewBuilder(builderOpts)

	// Build all message types and collect their descriptors
	var messageProtos []*descriptorpb.DescriptorProto
	processedTypes := make(map[string]bool)

	// Use a sorted order to ensure consistent output
	typeNames := make([]string, 0, len(messageTypes))
	for name := range messageTypes {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		typ := messageTypes[typeName]
		if processedTypes[typeName] {
			continue
		}

		// Build message using the file builder
		_, err := fileBuilder.BuildMessage(typ)
		if err != nil {
			// Log error but continue
			fmt.Printf("[WARNING] Failed to build message %s: %v\n", typeName, err)
			continue
		}
		processedTypes[typeName] = true
	}

	// Get all built files and extract message descriptors
	builtFiles := fileBuilder.GetFileDescriptorSet()
	if builtFiles != nil {
		// Collect all message types from all files
		allMessages := make(map[string]*descriptorpb.DescriptorProto)
		for _, file := range builtFiles.File {
			// Only include messages from our package
			if file.GetPackage() == s.packageName {
				for _, msg := range file.MessageType {
					// Avoid duplicates
					allMessages[msg.GetName()] = msg
				}
			}
		}

		// Convert map to slice
		for _, msg := range allMessages {
			messageProtos = append(messageProtos, msg)
		}
	}

	// Create service descriptor
	serviceProto := &descriptorpb.ServiceDescriptorProto{
		Name:   ptr(s.name),
		Method: []*descriptorpb.MethodDescriptorProto{},
	}

	// Add service comment if available
	if s.options.Description != "" {
		path := []int32{schema.FileDescriptorProtoServiceField, 0} // First service
		sourceCodeInfo.AddLocation(path, &schema.CommentInfo{
			Leading: s.options.Description,
		})
	}

	// Add method descriptors
	methodIndex := int32(0)
	for methodName, method := range s.methods {
		// Get type names
		inputTypeName := fmt.Sprintf(".%s.%s", s.packageName, method.InputType.Name())
		outputTypeName := fmt.Sprintf(".%s.%s", s.packageName, method.OutputType.Name())

		// Create method descriptor
		methodProto := &descriptorpb.MethodDescriptorProto{
			Name:       ptr(methodName),
			InputType:  ptr(inputTypeName),
			OutputType: ptr(outputTypeName),
		}
		serviceProto.Method = append(serviceProto.Method, methodProto)

		// Add method comment if available
		if method.Options.Description != "" {
			path := []int32{
				schema.FileDescriptorProtoServiceField, 0, // First service
				schema.ServiceDescriptorProtoMethodField, methodIndex,
			}
			sourceCodeInfo.AddLocation(path, &schema.CommentInfo{
				Leading: method.Options.Description,
			})
		}
		methodIndex++
	}

	// Create a single file with all messages and the service
	fileProto := &descriptorpb.FileDescriptorProto{
		Name:        ptr(fmt.Sprintf("%s.proto", s.packageName)),
		Package:     ptr(s.packageName),
		MessageType: messageProtos,
		Service:     []*descriptorpb.ServiceDescriptorProto{serviceProto},
	}

	// Add well-known type imports if needed
	// Check the built files for dependencies
	importMap := make(map[string]bool)
	if builtFiles != nil {
		for _, file := range builtFiles.File {
			for _, dep := range file.Dependency {
				if strings.HasPrefix(dep, "google/protobuf/") {
					importMap[dep] = true
				}
			}
		}
	}
	// Convert map to slice
	imports := make([]string, 0, len(importMap))
	for imp := range importMap {
		imports = append(imports, imp)
	}
	fileProto.Dependency = imports

	// Set syntax based on service options
	if s.options.UseEditions {
		fileProto.Syntax = ptr("editions")
		edition := s.options.Edition
		if edition == "" {
			edition = schema.Edition2023
		}
		fileProto.Edition = schema.StringToEdition(edition)
	} else {
		fileProto.Syntax = ptr("proto3")
	}

	// Attach source code info only if we have locations
	if sci := sourceCodeInfo.Build(); sci != nil {
		fileProto.SourceCodeInfo = sci
	}

	// Create complete FileDescriptorSet with just this single file
	fdset := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fileProto},
	}

	return fdset
}

// collectNestedTypes recursively collects all types referenced by a given type
func collectNestedTypes(t reflect.Type, collected map[string]reflect.Type, packageName string) {
	// Handle pointers, slices, arrays
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}

	// Handle maps
	if t.Kind() == reflect.Map {
		collectNestedTypes(t.Key(), collected, packageName)
		collectNestedTypes(t.Elem(), collected, packageName)
		return
	}

	// Only process structs
	if t.Kind() != reflect.Struct {
		return
	}

	// Skip if already collected or if it's a well-known type
	if _, exists := collected[t.Name()]; exists {
		return
	}

	// Skip well-known types
	if t.PkgPath() == "time" || strings.HasPrefix(t.PkgPath(), "google.golang.org/protobuf") {
		return
	}

	// Skip if not from the same package (to avoid pulling in external types)
	if t.PkgPath() != "" && !strings.Contains(t.PkgPath(), packageName) {
		return
	}

	// Add to collected
	if t.Name() != "" {
		collected[t.Name()] = t
	}

	// Process all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.IsExported() {
			collectNestedTypes(field.Type, collected, packageName)
		}
	}
}

// NewGateway creates a gateway for the service.
func NewGateway(services ...*Service) (http.Handler, error) {
	gatewaySvcs := make([]*gateway.Service, 0, len(services))

	for _, svc := range services {
		// Build handlers for each method
		handlers := make(map[string]http.Handler)

		// Build complete FileDescriptorSet for this service
		// This will create a single file with all messages and the service
		fdset := svc.buildCompleteFileDescriptorSet()

		// Create method handlers
		for _, method := range svc.methods {
			// Create handler path - use fully qualified service name
			path := fmt.Sprintf("/%s.%s/%s", svc.packageName, svc.name, method.Name)

			// Create actual handler for the method
			handlers[path] = svc.createHTTPHandler(method)
		}

		gatewaySvc := &gateway.Service{
			Name:        svc.name,
			Package:     svc.packageName,
			Handlers:    handlers,
			Descriptors: fdset,
		}
		gatewaySvcs = append(gatewaySvcs, gatewaySvc)
	}

	// Check if any service has reflection enabled
	enableReflection := false
	for _, svc := range services {
		if svc.options.EnableReflection {
			enableReflection = true
			break
		}
	}

	// Create gateway with options from services
	gw, err := gateway.New(gatewaySvcs, gateway.Options{
		EnableReflection: enableReflection,
		EnableOpenAPI:    true,
		OpenAPIPath:      "/openapi.json",
		CORSConfig:       gateway.DefaultCORSConfig(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	return gw, nil
}

// Register registers a typed method (recommended).
func Register[TIn, TOut any](svc *Service, name string, handler Handler[TIn, TOut]) error {
	method := NewMethod(name, handler)
	return svc.Register(method.Build())
}

// MustRegister registers a typed method and panics on error (recommended).
func MustRegister[TIn, TOut any](svc *Service, name string, handler Handler[TIn, TOut]) {
	if err := Register(svc, name, handler); err != nil {
		panic(err)
	}
}

// RegisterMethod registers a method using the builder pattern.
func RegisterMethod(svc *Service, methods ...*MethodBuilder) error {
	for _, mb := range methods {
		if err := svc.Register(mb.Build()); err != nil {
			return err
		}
	}
	return nil
}

// MustRegisterMethod registers methods using the builder pattern and panics on error.
func MustRegisterMethod(svc *Service, methods ...*MethodBuilder) {
	for _, mb := range methods {
		svc.MustRegister(mb.Build())
	}
}

// ptr is a helper to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}

// Name returns the service name.
func (s *Service) Name() string {
	return s.name
}

// PackageName returns the service package name.
func (s *Service) PackageName() string {
	return s.packageName
}

// Handlers returns the HTTP handlers for all methods.
func (s *Service) Handlers() map[string]http.Handler {
	handlers := make(map[string]http.Handler)
	for methodName, method := range s.methods {
		path := fmt.Sprintf("/%s.%s/%s", s.packageName, s.name, methodName)
		handlers[path] = s.createHTTPHandler(method)
	}
	return handlers
}

// WithInterceptors adds interceptors to the service.
func WithInterceptors(interceptors ...Interceptor) ServiceOption {
	return func(o *ServiceOptions) {
		o.Interceptors = append(o.Interceptors, interceptors...)
	}
}

// WithEdition enables Protobuf Editions mode with the specified edition.
func WithEdition(edition string) ServiceOption {
	return func(o *ServiceOptions) {
		o.UseEditions = true
		o.Edition = edition
	}
}

// WithServiceConfig sets the gRPC service configuration.
func WithServiceConfig(jsonConfig string) ServiceOption {
	return func(o *ServiceOptions) {
		o.ServiceConfig = jsonConfig
	}
}

// WithDescription sets the service description for documentation.
func WithDescription(description string) ServiceOption {
	return func(o *ServiceOptions) {
		o.Description = description
	}
}
