package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/i2y/hyperway/examples/connect-go-client/gen"
	"github.com/i2y/hyperway/examples/connect-go-client/gen/userv1connect"
)

func testProtocol(name string, options []connect.ClientOption) {
	fmt.Printf("%c=== Testing %s ===%c", 10, name, 10)

	client := userv1connect.NewUserServiceClient(
		http.DefaultClient,
		"http://localhost:8888",
		options...,
	)

	ctx := context.Background()

	// Create a user
	createReq := connect.NewRequest(&userv1.CreateUserRequest{
		Name:  fmt.Sprintf("Test User %s", name),
		Email: fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
	})

	start := time.Now()
	createResp, err := client.CreateUser(ctx, createReq)
	duration := time.Since(start)

	if err != nil {
		log.Printf("%s: CreateUser failed: %v", name, err)
		return
	}

	fmt.Printf("%s: Created user ID=%s in %v%c", name, createResp.Msg.User.Id, duration, 10)

	// Get the user
	getReq := connect.NewRequest(&userv1.GetUserRequest{
		Id: createResp.Msg.User.Id,
	})

	start = time.Now()
	_, err = client.GetUser(ctx, getReq)
	duration = time.Since(start)

	if err != nil {
		log.Printf("%s: GetUser failed: %v", name, err)
		return
	}

	fmt.Printf("%s: Retrieved user in %v%c", name, duration, 10)
	fmt.Printf("%s: Success - All operations completed%c", name, 10)
}

func main() {
	protocols := []struct {
		name    string
		options []connect.ClientOption
	}{
		{
			name:    "Connect+Protobuf",
			options: []connect.ClientOption{},
		},
		{
			name: "Connect+JSON",
			options: []connect.ClientOption{
				connect.WithProtoJSON(),
			},
		},
		{
			name: "gRPC-Web+Protobuf",
			options: []connect.ClientOption{
				connect.WithGRPCWeb(),
			},
		},
		{
			name: "gRPC-Web+JSON",
			options: []connect.ClientOption{
				connect.WithGRPCWeb(),
				connect.WithProtoJSON(),
			},
		},
	}

	fmt.Println("Testing all protocol combinations with Connect-go client...")

	for _, p := range protocols {
		testProtocol(p.name, p.options)
	}

	fmt.Printf("%c=== All protocol tests completed ===%c", 10, 10)
}
