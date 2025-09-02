package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Service request/response types
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
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
	PageSize  int32  `json:"page_size" validate:"min=0,max=100"`
	PageToken string `json:"page_token"`
}

type ListUsersResponse struct {
	Users         []*User `json:"users"`
	NextPageToken string  `json:"next_page_token"`
}

// Streaming request/response types
type StreamUsersRequest struct {
	BatchSize int32 `json:"batch_size"`
	DelayMs   int32 `json:"delay_ms"`
}

type BatchUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BatchCreateResponse struct {
	CreatedUsers []*User `json:"created_users"`
	TotalCreated int32   `json:"total_created"`
}

type ChatRequest struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type ChatResponse struct {
	ServerMessage string    `json:"server_message"`
	Timestamp     time.Time `json:"timestamp"`
}

// UserService implementation
type UserService struct {
	users map[string]*User
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[string]*User),
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	user := &User{
		ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}

	s.users[user.ID] = user

	return &CreateUserResponse{User: user}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	user, exists := s.users[req.ID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", req.ID)
	}

	return &GetUserResponse{User: user}, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	var users []*User
	for _, user := range s.users {
		users = append(users, user)
	}

	pageSize := req.PageSize
	if pageSize == 0 || pageSize > 100 {
		pageSize = 10
	}

	if int(pageSize) > len(users) {
		return &ListUsersResponse{Users: users}, nil
	}

	return &ListUsersResponse{
		Users:         users[:pageSize],
		NextPageToken: "next",
	}, nil
}

// StreamUsers implements server streaming - streams all users
func (s *UserService) StreamUsers(ctx context.Context, req *StreamUsersRequest, stream rpc.ServerStream[User]) error {

	// Get all users
	var users []*User
	for _, user := range s.users {
		users = append(users, user)
	}

	// Default batch size
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	// Stream users in batches
	for i := 0; i < len(users); i++ {
		if err := stream.Send(users[i]); err != nil {
			return err
		}

		// Add delay if requested
		if req.DelayMs > 0 && i < len(users)-1 {
			time.Sleep(time.Duration(req.DelayMs) * time.Millisecond)
		}
	}

	return nil
}

// BatchCreateUsers implements client streaming - batch create users
func (s *UserService) BatchCreateUsers(ctx context.Context, stream rpc.ClientStream[BatchUserRequest]) (*BatchCreateResponse, error) {

	var createdUsers []*User
	var count int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Client finished sending
			break
		}
		if err != nil {
			return nil, err
		}

		// Create user
		user := &User{
			ID:        fmt.Sprintf("batch-user-%d-%d", time.Now().UnixNano(), count),
			Name:      req.Name,
			Email:     req.Email,
			CreatedAt: time.Now(),
		}
		s.users[user.ID] = user
		createdUsers = append(createdUsers, user)
		count++

	}

	return &BatchCreateResponse{
		CreatedUsers: createdUsers,
		TotalCreated: count,
	}, nil
}

// UserChat implements bidirectional streaming - chat
func (s *UserService) UserChat(ctx context.Context, stream rpc.BidiStream[ChatRequest, ChatResponse]) error {

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Echo back with server response
		resp := &ChatResponse{
			ServerMessage: fmt.Sprintf("Server received from %s: %s", req.UserID, req.Message),
			Timestamp:     time.Now(),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func main() {
	// Create service
	userService := NewUserService()

	svc := rpc.NewService("UserService",
		rpc.WithPackage("user.v1"),
		rpc.WithValidation(true),
	)

	// Register methods
	svc.MustRegister("CreateUser", userService.CreateUser)
	svc.MustRegister("GetUser", userService.GetUser)
	svc.MustRegister("ListUsers", userService.ListUsers)

	// Register streaming methods
	svc.MustRegisterServerStream("StreamUsers", userService.StreamUsers)
	svc.MustRegisterClientStream("BatchCreateUsers", userService.BatchCreateUsers)
	svc.MustRegisterBidiStream("UserChat", userService.UserChat)

	// Export proto file if requested
	if len(os.Args) > 1 && os.Args[1] == "export-proto" {
		log.Println("Exporting proto file...")

		// Export proto files with Go package option for Connect-go code generation
		files, err := svc.ExportAllProtosWithOptions(
			proto.WithGoPackage("github.com/i2y/hyperway/examples/connect-go-client/gen;userv1"),
		)
		if err != nil {
			log.Fatalf("Failed to export proto: %v", err)
		}

		// Write proto files
		for filename, content := range files {
			// Create subdirectories if needed
			if dir := filepath.Dir(filename); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					log.Fatalf("Failed to create directory %s: %v", dir, err)
				}
			}

			if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
				log.Fatalf("Failed to write %s: %v", filename, err)
			}
			log.Printf("Exported: %s", filename)
		}

		log.Println("\nProto files exported successfully!")
		log.Println("Note: go_package option has been automatically added for Connect-go code generation")
		log.Println("You can now generate Connect-go client code with:")
		log.Println("  buf generate")
		return
	}

	// Create and start server
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	h2s := &http2.Server{}
	h2Handler := h2c.NewHandler(mux, h2s)

	server := &http.Server{
		Addr:         ":8888",
		Handler:      h2Handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Println("Starting Hyperway server on :8888")
	log.Println("Service: user.v1.UserService")
	log.Println("Protocols: gRPC, Connect, gRPC-Web")
	log.Println("")
	log.Println("To export proto file, run: go run main.go export-proto")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
