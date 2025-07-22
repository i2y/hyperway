// Package main demonstrates how to add comments to proto exports.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/i2y/hyperway/rpc"
)

// CreateUserRequest is the request for creating a new user.
// The struct demonstrates how to add proto documentation using tags.
type CreateUserRequest struct {
	// Special field for message-level documentation
	_ struct{} `protoDoc:"CreateUserRequest contains all the information needed to create a new user account in the system."`

	// User fields with documentation
	Email    string `json:"email" doc:"The user's email address. Must be unique across the system." validate:"required,email"`
	Username string `json:"username" doc:"The user's chosen username. Must be 3-20 characters, alphanumeric only." validate:"required,min=3,max=20,alphanum"`
	FullName string `json:"full_name" doc:"The user's full name for display purposes."`
	Age      *int32 `json:"age,omitempty" doc:"The user's age. Optional field, must be 13 or older if provided." validate:"omitempty,min=13"`

	// Deprecated field example
	OldUserID string `json:"old_user_id,omitempty" doc:"@deprecated Use username instead. This field will be removed in v2.0."`
}

// CreateUserResponse is the response after creating a user.
type CreateUserResponse struct {
	_ struct{} `protoDoc:"CreateUserResponse contains the newly created user's information and system-generated identifiers."`

	UserID    string `json:"user_id" doc:"The system-generated unique identifier for the user."`
	Username  string `json:"username" doc:"The user's username as stored in the system."`
	CreatedAt int64  `json:"created_at" doc:"Unix timestamp indicating when the user was created."`
	Status    string `json:"status" doc:"The user's account status. Possible values: 'active', 'pending', 'suspended'."`
}

// GetUserRequest retrieves a user by ID.
type GetUserRequest struct {
	_ struct{} `protoDoc:"GetUserRequest is used to retrieve user information by user ID."`

	UserID string `json:"user_id" doc:"The unique identifier of the user to retrieve." validate:"required"`
}

// GetUserResponse contains user information.
type GetUserResponse struct {
	_ struct{} `protoDoc:"GetUserResponse contains the complete user information retrieved from the system."`

	UserID    string `json:"user_id" doc:"The user's unique identifier."`
	Email     string `json:"email" doc:"The user's email address."`
	Username  string `json:"username" doc:"The user's username."`
	FullName  string `json:"full_name" doc:"The user's full name."`
	Age       *int32 `json:"age,omitempty" doc:"The user's age, if provided."`
	Status    string `json:"status" doc:"Current account status."`
	CreatedAt int64  `json:"created_at" doc:"Account creation timestamp (Unix time)."`
	UpdatedAt int64  `json:"updated_at" doc:"Last update timestamp (Unix time)."`
}

// UserMetadata contains additional user metadata.
type UserMetadata struct {
	_ struct{} `protoDoc:"UserMetadata stores additional optional information about a user that can be extended over time."`

	Preferences map[string]string `json:"preferences" doc:"User preferences as key-value pairs. Keys should be namespaced (e.g., 'ui.theme', 'notification.email')."`
	Tags        []string          `json:"tags" doc:"Tags associated with the user for categorization and searching."`
}

// Handler implementations
func CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	// Implementation would create user in database
	return &CreateUserResponse{
		UserID:    "usr_" + req.Username,
		Username:  req.Username,
		CreatedAt: 1234567890,
		Status:    "active",
	}, nil
}

func GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	// Implementation would fetch user from database
	return &GetUserResponse{
		UserID:    req.UserID,
		Email:     "user@example.com",
		Username:  "johndoe",
		FullName:  "John Doe",
		Status:    "active",
		CreatedAt: 1234567890,
		UpdatedAt: 1234567900,
	}, nil
}

func main() {
	// Create service with documentation
	svc := rpc.NewService("UserService",
		rpc.WithPackage("example.user.v1"),
		rpc.WithDescription("UserService manages user accounts and profiles in the system."),
		rpc.WithValidation(true),
	)

	// Register methods with documentation
	rpc.MustRegisterMethod(svc,
		rpc.NewMethod("CreateUser", CreateUser).
			WithDescription("CreateUser creates a new user account with the provided information. Returns an error if the email or username already exists."),
		rpc.NewMethod("GetUser", GetUser).
			WithDescription("GetUser retrieves user information by user ID. Returns NOT_FOUND if the user doesn't exist."),
	)

	// Export all proto files with comments
	protos, err := svc.ExportAllProtos()
	if err != nil {
		log.Fatalf("Failed to export protos: %v", err)
	}

	fmt.Println("Generated Proto Files with Comments:")
	fmt.Println("====================================")

	// Show service proto first
	for name, content := range protos {
		if strings.Contains(name, "UserService") || strings.Contains(name, "example.user.v1.proto") {
			fmt.Printf("\n=== %s ===\n", name)
			fmt.Println(content)
			break
		}
	}

	// Show message protos
	fmt.Println("\n=== Message Proto Files ===")
	for name, content := range protos {
		if strings.Contains(name, "request.proto") || strings.Contains(name, "response.proto") {
			fmt.Printf("\n=== %s ===\n", name)
			fmt.Println(content)
		}
	}

	// The exported proto will include:
	// - Service-level documentation
	// - Method-level documentation
	// - Message-level documentation (from protoDoc tags)
	// - Field-level documentation (from doc tags)
	// - Deprecation notices

	// Example output preview:
	/*
		syntax = "proto3";

		package example.user.v1;

		// UserService manages user accounts and profiles in the system.
		service UserService {
		  // CreateUser creates a new user account with the provided information. Returns an error if the email or username already exists.
		  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);

		  // GetUser retrieves user information by user ID. Returns NOT_FOUND if the user doesn't exist.
		  rpc GetUser(GetUserRequest) returns (GetUserResponse);
		}

		// CreateUserRequest contains all the information needed to create a new user account in the system.
		message CreateUserRequest {
		  // The user's email address. Must be unique across the system.
		  string email = 1;

		  // The user's chosen username. Must be 3-20 characters, alphanumeric only.
		  string username = 2;

		  // The user's full name for display purposes.
		  string full_name = 3;

		  // The user's age. Optional field, must be 13 or older if provided.
		  int32 age = 4;

		  // @deprecated Use username instead. This field will be removed in v2.0.
		  string old_user_id = 5;
		}
	*/
}
