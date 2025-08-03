// Package main demonstrates full gRPC protocol support with grpcurl compatibility.
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
	defaultLimit      = 10
	maxLimit          = 100
	httpReadTimeout   = 30 * time.Second
	httpWriteTimeout  = 30 * time.Second
	httpIdleTimeout   = 120 * time.Second
	httpHeaderTimeout = 5 * time.Second
)

// Service models
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required,min=3,max=50"`
	Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
	User *User `json:"user"`
}

type GetUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type GetUserResponse struct {
	User *User `json:"user"`
}

type ListUsersRequest struct {
	Limit  int32 `json:"limit,omitempty"`
	Offset int32 `json:"offset,omitempty"`
}

type ListUsersResponse struct {
	Users []User `json:"users"`
	Total int32  `json:"total"`
}

type DeleteUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type DeleteUserResponse struct {
	Success bool `json:"success"`
}

// In-memory storage for demo
var users = make(map[string]*User)
var nextID = 1

// Handler implementations
func createUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	user := &User{
		ID:        fmt.Sprintf("user-%d", nextID),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}
	users[user.ID] = user
	nextID++

	return &CreateUserResponse{
		User: user,
	}, nil
}

func getUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	user, ok := users[req.ID]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", req.ID)
	}

	return &GetUserResponse{
		User: user,
	}, nil
}

func listUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Convert map to slice
	allUsers := make([]User, 0, len(users))
	for _, user := range users {
		allUsers = append(allUsers, *user)
	}

	// Apply pagination
	start := int(offset)
	end := start + int(limit)
	if start > len(allUsers) {
		start = len(allUsers)
	}
	if end > len(allUsers) {
		end = len(allUsers)
	}

	return &ListUsersResponse{
		Users: allUsers[start:end],
		Total: int32(len(allUsers)), //nolint:gosec // Safe conversion, allUsers length is controlled
	}, nil
}

func deleteUser(ctx context.Context, req *DeleteUserRequest) (*DeleteUserResponse, error) {
	_, ok := users[req.ID]
	if !ok {
		return &DeleteUserResponse{
			Success: false,
		}, nil
	}

	delete(users, req.ID)

	return &DeleteUserResponse{
		Success: true,
	}, nil
}

func main() {
	// Create service with full gRPC support
	svc := rpc.NewService("UserService",
		rpc.WithPackage("grpc.example.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true), // Enable reflection for grpcurl
	)

	// Register all methods
	if err := rpc.Register(svc, "CreateUser", createUser); err != nil {
		log.Fatalf("Failed to register CreateUser: %v", err)
	}
	if err := rpc.Register(svc, "GetUser", getUser); err != nil {
		log.Fatalf("Failed to register GetUser: %v", err)
	}
	if err := rpc.Register(svc, "ListUsers", listUsers); err != nil {
		log.Fatalf("Failed to register ListUsers: %v", err)
	}
	if err := rpc.Register(svc, "DeleteUser", deleteUser); err != nil {
		log.Fatalf("Failed to register DeleteUser: %v", err)
	}

	// Create gateway with gRPC support
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Start HTTP/2 server with h2c for gRPC support
	srv := &http.Server{
		Addr:              ":9095",
		Handler:           h2c.NewHandler(gateway, &http2.Server{}),
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		ReadHeaderTimeout: httpHeaderTimeout,
	}

	// Add some initial data
	users["user-0"] = &User{
		ID:        "user-0",
		Name:      "Admin User",
		Email:     "admin@example.com",
		CreatedAt: time.Now(),
	}

	log.Println("gRPC server starting on :9095")
	log.Println("")
	log.Println("Test with grpcurl:")
	log.Println("")
	log.Println("# List available services")
	log.Println("grpcurl -plaintext localhost:9095 list")
	log.Println("")
	log.Println("# Describe the service")
	log.Println("grpcurl -plaintext localhost:9095 describe grpc.example.v1.UserService")
	log.Println("")
	log.Println("# Create a user")
	log.Println(`grpcurl -plaintext -d '{"name":"Alice","email":"alice@example.com"}' localhost:9095 grpc.example.v1.UserService/CreateUser`)
	log.Println("")
	log.Println("# List users")
	log.Println(`grpcurl -plaintext -d '{"limit":10}' localhost:9095 grpc.example.v1.UserService/ListUsers`)
	log.Println("")
	log.Println("# Get a specific user")
	log.Println(`grpcurl -plaintext -d '{"id":"user-0"}' localhost:9095 grpc.example.v1.UserService/GetUser`)
	log.Println("")

	log.Fatal(srv.ListenAndServe())
}
