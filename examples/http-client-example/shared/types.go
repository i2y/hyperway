package shared

import "time"

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

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

type UpdateUserRequest struct {
	ID    string `json:"id" validate:"required"`
	Name  string `json:"name" validate:"omitempty,min=1,max=100"`
	Email string `json:"email" validate:"omitempty,email"`
}

type UpdateUserResponse struct {
	User *User `json:"user"`
}

type DeleteUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type DeleteUserResponse struct {
	Success bool `json:"success"`
}
