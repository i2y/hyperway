package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/i2y/hyperway/examples/http-client-example/client"
	"github.com/i2y/hyperway/examples/http-client-example/shared"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	flag.Parse()

	userClient := client.NewHTTPUserServiceClient(*serverURL)
	ctx := context.Background()

	fmt.Println("=== HTTP Client for Hyperway Server ===")
	fmt.Println()

	fmt.Println("1. Creating a user...")
	createResp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
		Name:  "Alice Smith",
		Email: "alice@example.com",
	})
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}
	fmt.Printf("Created user: ID=%s, Name=%s, Email=%s\n",
		createResp.User.ID, createResp.User.Name, createResp.User.Email)

	userID := createResp.User.ID

	fmt.Println("\n2. Getting the user...")
	getResp, err := userClient.GetUser(ctx, &shared.GetUserRequest{
		ID: userID,
	})
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("Retrieved user: ID=%s, Name=%s, Email=%s\n",
		getResp.User.ID, getResp.User.Name, getResp.User.Email)

	fmt.Println("\n3. Creating another user...")
	createResp2, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
		Name:  "Bob Johnson",
		Email: "bob@example.com",
	})
	if err != nil {
		log.Fatalf("Failed to create second user: %v", err)
	}
	fmt.Printf("Created user: ID=%s, Name=%s\n",
		createResp2.User.ID, createResp2.User.Name)

	fmt.Println("\n4. Listing users...")
	listResp, err := userClient.ListUsers(ctx, &shared.ListUsersRequest{
		PageSize: 10,
	})
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	fmt.Printf("Found %d users:\n", len(listResp.Users))
	for _, user := range listResp.Users {
		fmt.Printf("  - ID=%s, Name=%s, Email=%s\n",
			user.ID, user.Name, user.Email)
	}

	fmt.Println("\n5. Updating user...")
	updateResp, err := userClient.UpdateUser(ctx, &shared.UpdateUserRequest{
		ID:    userID,
		Name:  "Alice Johnson",
		Email: "alice.johnson@example.com",
	})
	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}
	fmt.Printf("Updated user: Name=%s, Email=%s\n",
		updateResp.User.Name, updateResp.User.Email)

	fmt.Println("\n6. Deleting user...")
	deleteResp, err := userClient.DeleteUser(ctx, &shared.DeleteUserRequest{
		ID: userID,
	})
	if err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	}
	fmt.Printf("User deleted: %v\n", deleteResp.Success)

	fmt.Println("\n7. Trying to get deleted user (should fail)...")
	_, err = userClient.GetUser(ctx, &shared.GetUserRequest{
		ID: userID,
	})
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	} else {
		fmt.Println("Unexpected: User still exists!")
	}

	fmt.Println("\n=== All operations completed successfully ===")
}
