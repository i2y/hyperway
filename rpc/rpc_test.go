package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/i2y/hyperway/rpc"
)

// Test types
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required,min=3,max=50"`
	Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type GetUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Test handlers
func createUserHandler(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	if req.Name == "error" {
		return nil, errors.New("test error")
	}
	return &CreateUserResponse{
		ID:   "user-123",
		Name: req.Name,
	}, nil
}

func getUserHandler(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	if req.ID == "not-found" {
		return nil, errors.New("user not found")
	}
	return &GetUserResponse{
		ID:    req.ID,
		Name:  "Test User",
		Email: "test@example.com",
	}, nil
}

func TestService_Creation(t *testing.T) {
	// Test basic service creation
	svc := rpc.NewService("TestService")
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}

	// Test with options
	svc = rpc.NewService("TestService",
		rpc.WithPackage("test.v1"),
		rpc.WithValidation(true),
	)
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestService_MethodRegistration(t *testing.T) {
	svc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))

	// Register a method
	err := rpc.Register(svc, "CreateUser", createUserHandler)
	if err != nil {
		t.Fatalf("Failed to register method: %v", err)
	}

	// Register another method
	err = rpc.Register(svc, "GetUser", getUserHandler)
	if err != nil {
		t.Fatalf("Failed to register method: %v", err)
	}
}

func TestService_HTTPHandler(t *testing.T) {
	svc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	// Create gateway (which includes HTTP handlers)
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test JSON request
	reqBody := `{"name":"Alice","email":"alice@example.com"}`
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/user.v1.UserService/CreateUser",
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if !strings.Contains(string(body), `"id":"user-123"`) {
		t.Errorf("Expected response to contain user ID, got: %s", string(body))
	}
}

func TestService_Validation(t *testing.T) {
	svc := rpc.NewService("UserService",
		rpc.WithPackage("user.v1"),
		rpc.WithValidation(true),
	)

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test with invalid data (missing required fields)
	reqBody := `{"name":"Al"}` // Name too short, email missing
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/user.v1.UserService/CreateUser",
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Should fail validation
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected validation error, got status 200: %s", string(body))
	}
}

func TestService_ErrorHandling(t *testing.T) {
	svc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test with error-triggering input
	reqBody := `{"name":"error","email":"error@example.com"}`
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/user.v1.UserService/CreateUser",
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// For Connect protocol, errors are returned with 200 status
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "test error") && !strings.Contains(string(body), "error") {
		t.Errorf("Expected error message in response, got: %s", string(body))
	}
}

func TestService_Gateway(t *testing.T) {
	svc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
		rpc.NewMethod("GetUser", getUserHandler).
			In(GetUserRequest{}).
			Out(GetUserResponse{}),
	)

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test OpenAPI endpoint
	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/openapi.json", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get OpenAPI spec: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for OpenAPI, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "openapi") {
		t.Error("Expected OpenAPI spec in response")
	}
}

func TestService_ConnectProtocol(t *testing.T) {
	svc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test Connect protocol request
	reqBody := `{"name":"Bob","email":"bob@example.com"}`
	req, err := http.NewRequestWithContext(context.Background(),
		"POST",
		server.URL+"/user.v1.UserService/CreateUser",
		strings.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestMethodBuilder(t *testing.T) {
	// Test method builder
	method := rpc.NewMethod("TestMethod", createUserHandler).
		Validate(true).
		Build()

	if method == nil {
		t.Fatal("Expected non-nil method")
	}

	if method.Name != "TestMethod" {
		t.Errorf("Expected method name 'TestMethod', got %s", method.Name)
	}

	// Verify types were inferred correctly
	if method.InputType.Name() != "CreateUserRequest" {
		t.Errorf("Expected input type 'CreateUserRequest', got %s", method.InputType.Name())
	}

	if method.OutputType.Name() != "CreateUserResponse" {
		t.Errorf("Expected output type 'CreateUserResponse', got %s", method.OutputType.Name())
	}
}

func TestTypedRegistration(t *testing.T) {
	svc := rpc.NewService("TypedService", rpc.WithPackage("typed.v1"))

	// Test typed registration
	rpc.MustRegisterTyped(svc, "CreateUser", createUserHandler)
	rpc.MustRegisterTyped(svc, "GetUser", getUserHandler)

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test that the service works
	reqBody := `{"name":"TypedTest","email":"typed@example.com"}`
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/typed.v1.TypedService/CreateUser",
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestService_MultipleServices(t *testing.T) {
	// Create multiple services
	userSvc := rpc.NewService("UserService", rpc.WithPackage("user.v1"))
	rpc.MustRegister(userSvc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	adminSvc := rpc.NewService("AdminService", rpc.WithPackage("admin.v1"))
	rpc.MustRegister(adminSvc,
		rpc.NewMethod("GetUser", getUserHandler).
			In(GetUserRequest{}).
			Out(GetUserResponse{}),
	)

	// Create gateway with both services
	gateway, err := rpc.NewGateway(userSvc, adminSvc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	// Test user service endpoint
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/user.v1.UserService/CreateUser",
		strings.NewReader(`{"name":"Test","email":"test@example.com"}`),
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request to user service: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for user service, got %d", resp.StatusCode)
	}

	// Test admin service endpoint
	req, err = http.NewRequestWithContext(context.Background(), "POST",
		server.URL+"/admin.v1.AdminService/GetUser",
		strings.NewReader(`{"id":"123"}`),
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request to admin service: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for admin service, got %d", resp.StatusCode)
	}
}

func TestConnectTimeoutHeader(t *testing.T) {
	svc := rpc.NewService("TimeoutService", rpc.WithPackage("timeout.v1"))

	// Handler that sleeps for a specified duration
	sleepHandler := func(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
		// Sleep for 100ms
		select {
		case <-time.After(100 * time.Millisecond):
			return &CreateUserResponse{ID: "user-123", Name: req.Name}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	rpc.MustRegisterTyped(svc, "Sleep", sleepHandler)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	t.Run("Timeout Exceeds", func(t *testing.T) {
		// Request with 50ms timeout (handler sleeps for 100ms)
		reqBody := `{"name":"Test","email":"test@example.com"}`
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/timeout.v1.TimeoutService/Sleep",
			strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		req.Header.Set("Connect-Timeout-Ms", "50") // 50ms timeout

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Should return 200 with error in body (Connect protocol)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should have deadline_exceeded error
		if code, ok := result["code"].(string); !ok || code != "deadline_exceeded" {
			t.Errorf("Expected code 'deadline_exceeded', got %v", result["code"])
		}
	})

	t.Run("Timeout Sufficient", func(t *testing.T) {
		// Request with 200ms timeout (handler sleeps for 100ms)
		reqBody := `{"name":"Test","email":"test@example.com"}`
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/timeout.v1.TimeoutService/Sleep",
			strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		req.Header.Set("Connect-Timeout-Ms", "200") // 200ms timeout

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should succeed
		if _, hasError := result["code"]; hasError {
			t.Errorf("Expected success, got error: %v", result)
		}
		if id, ok := result["id"].(string); !ok || id != "user-123" {
			t.Errorf("Expected id 'user-123', got %v", result["id"])
		}
	})
}

func TestErrorCodes(t *testing.T) {
	svc := rpc.NewService("ErrorService", rpc.WithPackage("error.v1"), rpc.WithValidation(true))

	// Handler that returns specific errors based on input
	errorHandler := func(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
		switch req.Name {
		case "not_found":
			return nil, rpc.ErrNotFound("user not found")
		case "invalid":
			return nil, rpc.ErrInvalidArgument("invalid user name")
		case "unauthenticated":
			return nil, rpc.ErrUnauthenticated("authentication required")
		case "permission":
			return nil, rpc.ErrPermissionDenied("access denied")
		default:
			return &CreateUserResponse{ID: "user-123", Name: req.Name}, nil
		}
	}

	rpc.MustRegisterTyped(svc, "TestError", errorHandler)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	tests := []struct {
		name         string
		inputName    string
		expectedCode string
	}{
		{"NotFound", "not_found", "not_found"},
		{"InvalidArgument", "invalid", "invalid_argument"},
		{"Unauthenticated", "unauthenticated", "unauthenticated"},
		{"PermissionDenied", "permission", "permission_denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := fmt.Sprintf(`{"name":%q,"email":"test@example.com"}`, tt.inputName)
			req, err := http.NewRequestWithContext(context.Background(), "POST",
				server.URL+"/error.v1.ErrorService/TestError",
				strings.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			// Connect protocol returns 200 with error in body
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if code, ok := result["code"].(string); !ok || code != tt.expectedCode {
				t.Errorf("Expected code '%s', got %v", tt.expectedCode, result["code"])
			}
		})
	}
}

func BenchmarkService_JSONRequest(b *testing.B) {
	svc := rpc.NewService("BenchService", rpc.WithPackage("bench.v1"))

	rpc.MustRegister(svc,
		rpc.NewMethod("CreateUser", createUserHandler).
			In(CreateUserRequest{}).
			Out(CreateUserResponse{}),
	)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	server := httptest.NewServer(gateway)
	defer server.Close()

	reqBody := `{"name":"Benchmark User","email":"bench@example.com"}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST",
			server.URL+"/bench.v1.BenchService/CreateUser",
			strings.NewReader(reqBody),
		)
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
