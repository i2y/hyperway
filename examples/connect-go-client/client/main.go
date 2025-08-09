package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	userv1 "github.com/i2y/hyperway/examples/connect-go-client/gen"
	"github.com/i2y/hyperway/examples/connect-go-client/gen/userv1connect"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	flag.Parse()

	// Create Connect client
	client := userv1connect.NewUserServiceClient(
		http.DefaultClient,
		*serverURL,
	)

	ctx := context.Background()

	fmt.Println("=== Connect-go Client with Hyperway Server ===")
	fmt.Println("Using actual Connect-go generated client!")
	fmt.Println()

	// 1. Create a user
	fmt.Println("1. Creating a user...")
	createReq := connect.NewRequest(&userv1.CreateUserRequest{
		Name:  "Alice Smith",
		Email: "alice@example.com",
	})

	createResp, err := client.CreateUser(ctx, createReq)
	if err != nil {
		log.Fatalf("CreateUser failed: %v", err)
	}

	user := createResp.Msg.User
	fmt.Printf("Created user: ID=%s, Name=%s, Email=%s, CreatedAt=%v\n",
		user.Id, user.Name, user.Email, user.CreatedAt.AsTime())

	userID := user.Id

	// 2. Get the user
	fmt.Println("\n2. Getting the user...")
	getReq := connect.NewRequest(&userv1.GetUserRequest{
		Id: userID,
	})

	getResp, err := client.GetUser(ctx, getReq)
	if err != nil {
		log.Fatalf("GetUser failed: %v", err)
	}

	user = getResp.Msg.User
	fmt.Printf("Retrieved user: ID=%s, Name=%s, Email=%s\n",
		user.Id, user.Name, user.Email)

	// 3. Create another user
	fmt.Println("\n3. Creating another user...")
	createReq2 := connect.NewRequest(&userv1.CreateUserRequest{
		Name:  "Bob Johnson",
		Email: "bob@example.com",
	})

	createResp2, err := client.CreateUser(ctx, createReq2)
	if err != nil {
		log.Fatalf("CreateUser failed: %v", err)
	}

	fmt.Printf("Created user: ID=%s, Name=%s\n",
		createResp2.Msg.User.Id, createResp2.Msg.User.Name)

	// 4. List users
	fmt.Println("\n4. Listing users...")
	listReq := connect.NewRequest(&userv1.ListUsersRequest{
		PageSize: 10,
	})

	listResp, err := client.ListUsers(ctx, listReq)
	if err != nil {
		log.Fatalf("ListUsers failed: %v", err)
	}

	fmt.Printf("Found %d users:\n", len(listResp.Msg.Users))
	for _, u := range listResp.Msg.Users {
		fmt.Printf("  - ID=%s, Name=%s, Email=%s, CreatedAt=%v\n",
			u.Id, u.Name, u.Email, u.CreatedAt.AsTime())
	}

	// 5. Test with headers (Connect protocol feature)
	fmt.Println("\n5. Testing with custom headers...")
	createReq3 := connect.NewRequest(&userv1.CreateUserRequest{
		Name:  "Charlie Brown",
		Email: "charlie@example.com",
	})
	createReq3.Header().Set("X-Custom-Header", "test-value")

	createResp3, err := client.CreateUser(ctx, createReq3)
	if err != nil {
		log.Fatalf("CreateUser with headers failed: %v", err)
	}

	fmt.Printf("Created user with headers: ID=%s, Name=%s\n",
		createResp3.Msg.User.Id, createResp3.Msg.User.Name)

	// Check response headers
	if createResp3.Header().Get("X-Response-Header") != "" {
		fmt.Printf("Got response header: %s\n", createResp3.Header().Get("X-Response-Header"))
	}

	fmt.Println("\n=== All operations completed successfully! ===")
	fmt.Println("This is a real Connect-go client communicating with Hyperway server!")
}
