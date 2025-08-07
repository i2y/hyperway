package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/i2y/hyperway/rpc"
)

// Define request/response types
type HelloRequest struct {
	Name string `json:"name"`
}

type HelloResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// Handler that accesses context values
func sayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	// Try to get values from context
	requestID, _ := ctx.Value("request-id").(string)
	userID, _ := ctx.Value("user-id").(string)

	return &HelloResponse{
		Message:   fmt.Sprintf("Hello, %s!", req.Name),
		RequestID: requestID,
		UserID:    userID,
	}, nil
}

// Middleware that adds values to context
func contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add request ID to context
		ctx := context.WithValue(r.Context(), "request-id", "req-123")
		
		// Add user ID from header to context
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			ctx = context.WithValue(ctx, "user-id", userID)
		}

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func main() {
	// Create service
	svc := rpc.NewService("GreeterService",
		rpc.WithPackage("example.v1"),
		rpc.WithValidation(true),
	)

	// Register handler
	if err := rpc.Register(svc, "SayHello", sayHello); err != nil {
		log.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Wrap gateway with context middleware
	handler := contextMiddleware(gateway)

	// Start server
	log.Println("Starting server on :8095")
	log.Println("Test with:")
	log.Println(`curl -X POST http://localhost:8095/example.v1.GreeterService/SayHello \`)
	log.Println(`  -H "Content-Type: application/json" \`)
	log.Println(`  -H "X-User-ID: user-456" \`)
	log.Println(`  -d '{"name":"Alice"}'`)
	
	log.Fatal(http.ListenAndServe(":8095", handler))
}