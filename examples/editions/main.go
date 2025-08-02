// Package main demonstrates how to use Protobuf Editions with hyperway.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

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

// UserRequest represents a user creation request.
type UserRequest struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
	Age   *int32 `json:"age,omitempty"` // Optional field - will have explicit presence in Editions
}

// UserResponse represents a user creation response.
type UserResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// CreateUser handles user creation requests.
func CreateUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	msg := fmt.Sprintf("User %s created", req.Name)
	if req.Age != nil {
		msg += fmt.Sprintf(" (age: %d)", *req.Age)
	}

	return &UserResponse{
		ID:      "user-123",
		Message: msg,
	}, nil
}

func main() {
	// Create a service using Protobuf Editions 2023
	svc := rpc.NewService("UserService",
		rpc.WithPackage("example.user.v1"),
		rpc.WithEdition("2023"), // Enable Editions mode
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Register the CreateUser method
	if err := rpc.Register(svc, "CreateUser", CreateUser); err != nil {
		log.Fatalf("Failed to register CreateUser: %v", err)
	}

	// Export the proto definition to see the editions syntax
	protoContent, err := svc.ExportProto()
	if err != nil {
		log.Fatalf("Failed to export proto: %v", err)
	}

	fmt.Println("Generated Proto with Editions syntax:")
	fmt.Println("=====================================")
	fmt.Println(protoContent)
	fmt.Println("=====================================")

	// Create HTTP gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Start server with h2c for both HTTP/1.1 and HTTP/2 support
	h2s := &http2.Server{}
	handler := h2c.NewHandler(gateway, h2s)

	fmt.Println("\nServer running on http://localhost:8080")
	fmt.Println("Try: curl -X POST http://localhost:8080/example.user.v1.UserService/CreateUser -d '{\"name\":\"Alice\",\"email\":\"alice@example.com\",\"age\":30}'")

	server := &http.Server{
		Addr:              ":8080",
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

/*
Key differences with Protobuf Editions:

1. Syntax Declaration:
   - Proto3: syntax = "proto3";
   - Editions: edition = "2023";

2. Field Presence:
   - Proto3: Uses proto3_optional for explicit presence on singular fields
   - Editions: Field presence is controlled by features (default is explicit presence)

3. Feature-based Configuration:
   - Editions uses features to control behavior instead of syntax-specific rules
   - Features include: field_presence, repeated_field_encoding, enum_type, utf8_validation

4. Future Compatibility:
   - Editions allow smooth evolution of the protocol without breaking changes
   - New features can be added without changing the syntax
*/
