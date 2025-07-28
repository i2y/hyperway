// Package gateway provides multi-protocol support for gRPC and Connect RPC.
package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/schema"
)

// Gateway wraps HTTP handlers for multi-protocol support.
type Gateway struct {
	handler    http.Handler
	services   []*Service
	options    Options
	descriptor *descriptorpb.FileDescriptorSet
	openAPI    []byte // Cached OpenAPI JSON
}

// Options configures the gateway.
type Options struct {
	// EnableReflection enables gRPC reflection
	EnableReflection bool
	// EnableOpenAPI enables OpenAPI endpoint
	EnableOpenAPI bool
	// OpenAPIPath is the path to serve OpenAPI spec
	OpenAPIPath string
	// CORSConfig configures CORS
	CORSConfig *CORSConfig
	// KeepaliveParams configures client-side keepalive
	KeepaliveParams *KeepaliveParameters
	// KeepaliveEnforcementPolicy configures server-side keepalive enforcement
	KeepaliveEnforcementPolicy *KeepaliveEnforcementPolicy
}

// CORSConfig configures CORS settings.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// Service represents a service with its handlers.
type Service struct {
	Name        string
	Package     string
	Handlers    map[string]http.Handler
	Descriptors *descriptorpb.FileDescriptorSet
}

// New creates a new gateway.
func New(services []*Service, opts Options) (*Gateway, error) {
	if opts.OpenAPIPath == "" {
		opts.OpenAPIPath = "/openapi.json"
	}

	// Build FileDescriptorSet from all services
	fdset := &descriptorpb.FileDescriptorSet{}
	for _, svc := range services {
		if svc.Descriptors != nil {
			fdset.File = append(fdset.File, svc.Descriptors.File...)
		}
	}

	// Create a custom router that handles all protocols
	// Store handlers in a map for direct lookup
	handlers := make(map[string]http.Handler)
	for _, svc := range services {
		for path, handler := range svc.Handlers {
			handlers[path] = handler
		}
	}

	gw := &Gateway{
		handler:    nil, // Will be set later
		services:   services,
		options:    opts,
		descriptor: fdset,
	}

	// Add reflection handlers if enabled
	if opts.EnableReflection {
		reflectionHandlers, err := gw.CreateReflectionHandlers()
		if err != nil {
			return nil, fmt.Errorf("failed to create reflection handlers: %w", err)
		}

		// Register reflection handlers in our handler map
		for path, handler := range reflectionHandlers {
			handlers[path] = handler
		}
	}

	// Wrap the mux to add multi-protocol support headers
	multiProtocolHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers for gRPC-Web (match reference server)
		// Important: Use Add instead of Set to preserve existing headers
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Add("Access-Control-Allow-Origin", origin)
			w.Header().Add("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Access-Control-Allow-Methods", "HEAD, GET, POST, PUT, PATCH, DELETE")
			w.Header().Add("Access-Control-Allow-Headers", "*")
			w.Header().Add("Access-Control-Expose-Headers", "*")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Look up the handler directly
		handler, found := handlers[r.URL.Path]
		if !found {
			// Try to find a handler by prefix (for services like reflection that handle multiple methods)
			for path, h := range handlers {
				if strings.HasSuffix(path, "/") && strings.HasPrefix(r.URL.Path, path) {
					handler = h
					found = true
					break
				}
			}

			if !found {
				// No handler found - return appropriate error for the protocol
				handleUnimplemented(w, r)
				return
			}
		}

		// Check if this is a gRPC-Web request and wrap the handler if needed
		if isGRPCWeb(r) {
			// Create a simple mux with just this handler for gRPC-Web
			tempMux := http.NewServeMux()
			tempMux.Handle(r.URL.Path, handler)
			webHandler := newGRPCWebHandler(tempMux, 30*time.Second)
			webHandler.ServeHTTP(w, r)
			return
		}

		// Serve the request directly
		handler.ServeHTTP(w, r)
	})

	gw.handler = multiProtocolHandler

	// Generate OpenAPI if enabled
	if opts.EnableOpenAPI {
		info := OpenAPIInfo{
			Title:   "Hyperway API",
			Version: "1.0.0",
		}

		spec, err := GenerateOpenAPI(fdset, info)
		if err != nil {
			return nil, fmt.Errorf("failed to generate OpenAPI: %w", err)
		}

		gw.openAPI, err = MarshalOpenAPI(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal OpenAPI: %w", err)
		}
	}

	return gw, nil
}

// ServeHTTP implements http.Handler.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CORS if configured
	if g.options.CORSConfig != nil {
		g.handleCORS(w, r)
		if r.Method == http.MethodOptions {
			return
		}
	}

	// Handle OpenAPI endpoint
	if g.options.EnableOpenAPI && r.URL.Path == g.options.OpenAPIPath {
		g.serveOpenAPI(w, r)
		return
	}

	// Handle proto export endpoints
	// Only match exact paths for proto export, not all paths starting with /proto
	if r.URL.Path == "/proto" || r.URL.Path == "/proto/" || r.URL.Path == "/proto.zip" || strings.HasPrefix(r.URL.Path, "/proto/") {
		g.serveProtoExport(w, r)
		return
	}

	// Pass to handler
	g.handler.ServeHTTP(w, r)
}

// handleCORS handles CORS headers.
func (g *Gateway) handleCORS(w http.ResponseWriter, r *http.Request) {
	cfg := g.options.CORSConfig

	// Set allowed origin
	origin := r.Header.Get("Origin")
	if len(cfg.AllowedOrigins) > 0 {
		for _, allowed := range cfg.AllowedOrigins {
			if allowed == "*" || allowed == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}

	// Set other CORS headers
	if len(cfg.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", joinStrings(cfg.AllowedMethods))
	}
	if len(cfg.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", joinStrings(cfg.AllowedHeaders))
	}
	if cfg.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if cfg.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
	}
}

// serveOpenAPI serves the OpenAPI specification.
func (g *Gateway) serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.openAPI != nil {
		_, _ = w.Write(g.openAPI)
	} else {
		_, _ = w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"Hyperway API","version":"1.0.0"}}`))
	}
}

// joinStrings joins strings with comma.
func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// ServiceBuilder helps build services.
type ServiceBuilder struct {
	name        string
	packageName string
	handlers    map[string]http.Handler
	builder     *schema.Builder
}

// NewServiceBuilder creates a new service builder.
func NewServiceBuilder(name, packageName string) *ServiceBuilder {
	return &ServiceBuilder{
		name:        name,
		packageName: packageName,
		handlers:    make(map[string]http.Handler),
		builder: schema.NewBuilder(schema.BuilderOptions{
			PackageName: packageName,
		}),
	}
}

// AddHandler adds a handler to the service.
func (sb *ServiceBuilder) AddHandler(path string, handler http.Handler) *ServiceBuilder {
	sb.handlers[path] = handler
	return sb
}

// Build creates the service.
func (sb *ServiceBuilder) Build() (*Service, error) {
	// Build FileDescriptorSet from registered types
	// This is simplified - in real implementation, we'd track types from handlers
	fdset := &descriptorpb.FileDescriptorSet{}

	return &Service{
		Name:        sb.name,
		Package:     sb.packageName,
		Handlers:    sb.handlers,
		Descriptors: fdset,
	}, nil
}

// DefaultCORSConfig returns a permissive CORS configuration for development.
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           24 * 60 * 60, // 24 hours in seconds
	}
}

// handleUnimplemented returns appropriate unimplemented error based on protocol
func handleUnimplemented(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// Detect protocol
	if strings.HasPrefix(contentType, "application/grpc") {
		// gRPC protocol
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("grpc-status", "12") // UNIMPLEMENTED
		w.Header().Set("grpc-message", "Method not found")
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.Contains(contentType, "connect") || r.Header.Get("Connect-Protocol-Version") == "1" {
		// Connect protocol
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"code":"unimplemented","message":"Method not found"}`)
		return
	}

	// Default HTTP 404
	http.NotFound(w, r)
}
