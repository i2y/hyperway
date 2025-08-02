package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/i2y/hyperway/gateway"
	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Constants
const (
	httpReadTimeout   = 30 * time.Second
	httpWriteTimeout  = 30 * time.Second
	httpIdleTimeout   = 120 * time.Second
	httpHeaderTimeout = 5 * time.Second
)

// Request and response types
type GreetRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type GreetResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type CalculateRequest struct {
	A        float64 `json:"a" validate:"required"`
	B        float64 `json:"b" validate:"required"`
	Operator string  `json:"operator" validate:"required,oneof=+ - * /"`
}

type CalculateResponse struct {
	Result float64 `json:"result"`
	Error  string  `json:"error,omitempty"`
}

// Service handlers
type GreeterService struct {
	validator *validator.Validate
}

func NewGreeterService() *GreeterService {
	return &GreeterService{
		validator: validator.New(),
	}
}

func (s *GreeterService) Greet(ctx context.Context, req *GreetRequest) (*GreetResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &GreetResponse{
		Message:   fmt.Sprintf("Hello, %s! Welcome to gRPC-Web with Hyperway!", req.Name),
		Timestamp: time.Now(),
	}, nil
}

func (s *GreeterService) Calculate(ctx context.Context, req *CalculateRequest) (*CalculateResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	var result float64
	var calcError string

	switch req.Operator {
	case "+":
		result = req.A + req.B
	case "-":
		result = req.A - req.B
	case "*":
		result = req.A * req.B
	case "/":
		if req.B == 0 {
			calcError = "division by zero"
		} else {
			result = req.A / req.B
		}
	default:
		calcError = "unknown operator"
	}

	return &CalculateResponse{
		Result: result,
		Error:  calcError,
	}, nil
}

// Static file server for the HTML client
func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "index.html")
		return
	}
	http.NotFound(w, r)
}

func main() {
	// Create service instance
	greeterService := NewGreeterService()

	// Create RPC service
	svc := rpc.NewService("GreeterService",
		rpc.WithPackage("grpcweb.example.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Register methods
	if err := rpc.Register(svc, "Greet", greeterService.Greet); err != nil {
		log.Fatalf("Failed to register Greet: %v", err)
	}
	if err := rpc.Register(svc, "Calculate", greeterService.Calculate); err != nil {
		log.Fatalf("Failed to register Calculate: %v", err)
	}

	// Create gateway with gRPC-Web support
	gw, err := gateway.New([]*gateway.Service{
		{
			Name:     "GreeterService",
			Package:  "grpcweb.example.v1",
			Handlers: svc.Handlers(),
		},
	}, gateway.Options{
		EnableReflection: true,
		EnableOpenAPI:    true,
		CORSConfig:       gateway.DefaultCORSConfig(),
	})
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Create HTTP mux
	mux := http.NewServeMux()

	// Handle static files
	mux.HandleFunc("/", serveStaticFiles)

	// Handle RPC endpoints (gRPC, gRPC-Web, Connect)
	mux.Handle("/grpcweb.example.v1.GreeterService/", gw)

	// Handle OpenAPI endpoint
	mux.Handle("/openapi.json", gw)

	// Start server
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("Starting gRPC-Web example server on %s", addr)
	log.Printf("Open http://localhost%s in your browser to test the gRPC-Web client", addr)
	log.Printf("Available endpoints:")
	log.Printf("  - /grpcweb.example.v1.GreeterService/Greet")
	log.Printf("  - /grpcweb.example.v1.GreeterService/Calculate")
	log.Printf("  - /openapi.json (OpenAPI specification)")

	// Create h2c handler to support HTTP/2 cleartext
	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		ReadHeaderTimeout: httpHeaderTimeout,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
