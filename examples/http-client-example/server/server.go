package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/i2y/hyperway/examples/http-client-example/shared"
)

type UserService struct {
	users map[string]*shared.User
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[string]*shared.User),
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *shared.CreateUserRequest) (*shared.CreateUserResponse, error) {
	user := &shared.User{
		ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}

	s.users[user.ID] = user

	log.Printf("Created user: %s", user.ID)
	return &shared.CreateUserResponse{User: user}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *shared.GetUserRequest) (*shared.GetUserResponse, error) {
	user, exists := s.users[req.ID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", req.ID)
	}

	return &shared.GetUserResponse{User: user}, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *shared.ListUsersRequest) (*shared.ListUsersResponse, error) {
	var users []*shared.User
	for _, user := range s.users {
		users = append(users, user)
	}

	pageSize := req.PageSize
	if pageSize == 0 || pageSize > 100 {
		pageSize = 10
	}

	if int(pageSize) > len(users) {
		return &shared.ListUsersResponse{Users: users}, nil
	}

	return &shared.ListUsersResponse{
		Users:         users[:pageSize],
		NextPageToken: "next",
	}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *shared.UpdateUserRequest) (*shared.UpdateUserResponse, error) {
	user, exists := s.users[req.ID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", req.ID)
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	log.Printf("Updated user: %s", user.ID)
	return &shared.UpdateUserResponse{User: user}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *shared.DeleteUserRequest) (*shared.DeleteUserResponse, error) {
	if _, exists := s.users[req.ID]; !exists {
		return nil, fmt.Errorf("user not found: %s", req.ID)
	}

	delete(s.users, req.ID)
	log.Printf("Deleted user: %s", req.ID)
	return &shared.DeleteUserResponse{Success: true}, nil
}
