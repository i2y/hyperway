package main_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/i2y/hyperway/examples/http-client-example/client"
	"github.com/i2y/hyperway/examples/http-client-example/server"
	"github.com/i2y/hyperway/examples/http-client-example/shared"
	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func setupTestServer(t *testing.T) *httptest.Server {
	userService := server.NewUserService()

	svc := rpc.NewService("user.v1",
		rpc.WithValidation(true),
	)

	rpc.MustRegister(svc, "CreateUser", userService.CreateUser)
	rpc.MustRegister(svc, "GetUser", userService.GetUser)
	rpc.MustRegister(svc, "ListUsers", userService.ListUsers)
	rpc.MustRegister(svc, "UpdateUser", userService.UpdateUser)
	rpc.MustRegister(svc, "DeleteUser", userService.DeleteUser)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	return testServer
}

func TestUserServiceIntegration(t *testing.T) {
	server := setupTestServer(t)
	userClient := client.NewHTTPUserServiceClient(server.URL)
	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		resp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
			Name:  "Test User",
			Email: "test@example.com",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		if resp.User == nil {
			t.Fatal("Expected user to be created")
		}
		if resp.User.Name != "Test User" {
			t.Errorf("Expected name 'Test User', got %s", resp.User.Name)
		}
		if resp.User.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %s", resp.User.Email)
		}
	})

	t.Run("CreateAndGet", func(t *testing.T) {
		createResp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
			Name:  "Alice",
			Email: "alice@test.com",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		getResp, err := userClient.GetUser(ctx, &shared.GetUserRequest{
			ID: createResp.User.ID,
		})
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}

		if getResp.User.ID != createResp.User.ID {
			t.Errorf("User ID mismatch: expected %s, got %s",
				createResp.User.ID, getResp.User.ID)
		}
		if getResp.User.Name != "Alice" {
			t.Errorf("Expected name 'Alice', got %s", getResp.User.Name)
		}
	})

	t.Run("ListUsers", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			_, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
				Name:  fmt.Sprintf("User %d", i),
				Email: fmt.Sprintf("user%d@test.com", i),
			})
			if err != nil {
				t.Fatalf("Failed to create user %d: %v", i, err)
			}
		}

		listResp, err := userClient.ListUsers(ctx, &shared.ListUsersRequest{
			PageSize: 10,
		})
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}

		if len(listResp.Users) < 3 {
			t.Errorf("Expected at least 3 users, got %d", len(listResp.Users))
		}
	})

	t.Run("UpdateUser", func(t *testing.T) {
		createResp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
			Name:  "Bob",
			Email: "bob@test.com",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		updateResp, err := userClient.UpdateUser(ctx, &shared.UpdateUserRequest{
			ID:    createResp.User.ID,
			Name:  "Robert",
			Email: "robert@test.com",
		})
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		if updateResp.User.Name != "Robert" {
			t.Errorf("Expected updated name 'Robert', got %s", updateResp.User.Name)
		}
		if updateResp.User.Email != "robert@test.com" {
			t.Errorf("Expected updated email 'robert@test.com', got %s", updateResp.User.Email)
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		createResp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
			Name:  "ToDelete",
			Email: "delete@test.com",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		deleteResp, err := userClient.DeleteUser(ctx, &shared.DeleteUserRequest{
			ID: createResp.User.ID,
		})
		if err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		if !deleteResp.Success {
			t.Error("Expected delete to succeed")
		}

		_, err = userClient.GetUser(ctx, &shared.GetUserRequest{
			ID: createResp.User.ID,
		})
		if err == nil {
			t.Error("Expected GetUser to fail after deletion")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		_, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
			Name:  "",
			Email: "invalid-email",
		})
		if err == nil {
			t.Error("Expected validation error for invalid email")
		}

		_, err = userClient.GetUser(ctx, &shared.GetUserRequest{
			ID: "",
		})
		if err == nil {
			t.Error("Expected validation error for empty ID")
		}
	})

	t.Run("NotFoundError", func(t *testing.T) {
		_, err := userClient.GetUser(ctx, &shared.GetUserRequest{
			ID: "non-existent-id",
		})
		if err == nil {
			t.Error("Expected error for non-existent user")
		}

		// For plain HTTP client, we just check that we got an error
		// The error format will be different from Connect errors
	})
}

func TestConcurrentOperations(t *testing.T) {
	server := setupTestServer(t)
	userClient := client.NewHTTPUserServiceClient(server.URL)
	ctx := context.Background()

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			resp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
				Name:  fmt.Sprintf("Concurrent User %d", id),
				Email: fmt.Sprintf("concurrent%d@test.com", id),
			})
			if err != nil {
				t.Errorf("Goroutine %d: CreateUser failed: %v", id, err)
				return
			}

			_, err = userClient.GetUser(ctx, &shared.GetUserRequest{
				ID: resp.User.ID,
			})
			if err != nil {
				t.Errorf("Goroutine %d: GetUser failed: %v", id, err)
			}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}

	listResp, err := userClient.ListUsers(ctx, &shared.ListUsersRequest{
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if len(listResp.Users) < numGoroutines {
		t.Errorf("Expected at least %d users, got %d", numGoroutines, len(listResp.Users))
	}
}

func BenchmarkCreateUser(b *testing.B) {
	userService := server.NewUserService()

	svc := rpc.NewService("user.v1",
		rpc.WithValidation(true),
	)

	rpc.MustRegister(svc, "CreateUser", userService.CreateUser)
	rpc.MustRegister(svc, "GetUser", userService.GetUser)
	rpc.MustRegister(svc, "ListUsers", userService.ListUsers)
	rpc.MustRegister(svc, "UpdateUser", userService.UpdateUser)
	rpc.MustRegister(svc, "DeleteUser", userService.DeleteUser)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	userClient := client.NewHTTPUserServiceClient(testServer.URL)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
				Name:  fmt.Sprintf("Bench User %d", i),
				Email: fmt.Sprintf("bench%d@test.com", i),
			})
			if err != nil {
				b.Fatalf("CreateUser failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkGetUser(b *testing.B) {
	userService := server.NewUserService()

	svc := rpc.NewService("user.v1",
		rpc.WithValidation(true),
	)

	rpc.MustRegister(svc, "CreateUser", userService.CreateUser)
	rpc.MustRegister(svc, "GetUser", userService.GetUser)
	rpc.MustRegister(svc, "ListUsers", userService.ListUsers)
	rpc.MustRegister(svc, "UpdateUser", userService.UpdateUser)
	rpc.MustRegister(svc, "DeleteUser", userService.DeleteUser)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	userClient := client.NewHTTPUserServiceClient(testServer.URL)
	ctx := context.Background()

	resp, err := userClient.CreateUser(ctx, &shared.CreateUserRequest{
		Name:  "Benchmark User",
		Email: "benchmark@test.com",
	})
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	userID := resp.User.ID

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := userClient.GetUser(ctx, &shared.GetUserRequest{
				ID: userID,
			})
			if err != nil {
				b.Fatalf("GetUser failed: %v", err)
			}
		}
	})
}
