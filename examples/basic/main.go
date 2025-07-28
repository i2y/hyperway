package main

import (
	"context"
	"log"
	"net/http"

	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Model definitions
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type GetUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type GetUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Handler implementations
func createUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	// In a real application, this would save to a database
	return &CreateUserResponse{
		ID:    "user-123",
		Name:  req.Name,
		Email: req.Email,
	}, nil
}

func getUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	// In a real application, this would fetch from a database
	return &GetUserResponse{
		ID:    req.ID,
		Name:  "John Doe",
		Email: "john@example.com",
	}, nil
}

func main() {
	// Create service
	svc := rpc.NewService("UserService",
		rpc.WithPackage("user.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Register methods - types are automatically inferred!
	if err := rpc.Register(svc, "CreateUser", createUser); err != nil {
		log.Fatalf("Failed to register CreateUser: %v", err)
	}
	if err := rpc.Register(svc, "GetUser", getUser); err != nil {
		log.Fatalf("Failed to register GetUser: %v", err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	// Start server
	log.Println("Server starting on :8091")
	log.Println("OpenAPI spec available at http://localhost:8091/openapi.json")
	log.Println("Example requests:")
	log.Println("  Create user: curl -X POST http://localhost:8091/user.v1.UserService/CreateUser -H 'Content-Type: application/json' -d '{\"name\":\"Alice\",\"email\":\"alice@example.com\"}'")
	log.Println("  Get user: curl -X POST http://localhost:8091/user.v1.UserService/GetUser -H 'Content-Type: application/json' -d '{\"id\":\"user-123\"}'")

	// Use h2c (HTTP/2 without TLS) for gRPC reflection support
	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	server := &http.Server{
		Addr:    ":8091",
		Handler: handler,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
