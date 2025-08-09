package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/i2y/hyperway/examples/http-client-example/shared"
)

// HTTPUserServiceClient is a simple HTTP client for the User Service
type HTTPUserServiceClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewHTTPUserServiceClient creates a new HTTP client for the User Service
func NewHTTPUserServiceClient(baseURL string) *HTTPUserServiceClient {
	return &HTTPUserServiceClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

func (c *HTTPUserServiceClient) doRequest(ctx context.Context, method string, req interface{}, resp interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/user.v1.user.v1/%s", c.baseURL, method)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var errResp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("%s: %s", errResp.Code, errResp.Message)
		}
		return fmt.Errorf("request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func (c *HTTPUserServiceClient) CreateUser(ctx context.Context, req *shared.CreateUserRequest) (*shared.CreateUserResponse, error) {
	resp := &shared.CreateUserResponse{}
	err := c.doRequest(ctx, "CreateUser", req, resp)
	return resp, err
}

func (c *HTTPUserServiceClient) GetUser(ctx context.Context, req *shared.GetUserRequest) (*shared.GetUserResponse, error) {
	resp := &shared.GetUserResponse{}
	err := c.doRequest(ctx, "GetUser", req, resp)
	return resp, err
}

func (c *HTTPUserServiceClient) ListUsers(ctx context.Context, req *shared.ListUsersRequest) (*shared.ListUsersResponse, error) {
	resp := &shared.ListUsersResponse{}
	err := c.doRequest(ctx, "ListUsers", req, resp)
	return resp, err
}

func (c *HTTPUserServiceClient) UpdateUser(ctx context.Context, req *shared.UpdateUserRequest) (*shared.UpdateUserResponse, error) {
	resp := &shared.UpdateUserResponse{}
	err := c.doRequest(ctx, "UpdateUser", req, resp)
	return resp, err
}

func (c *HTTPUserServiceClient) DeleteUser(ctx context.Context, req *shared.DeleteUserRequest) (*shared.DeleteUserResponse, error) {
	resp := &shared.DeleteUserResponse{}
	err := c.doRequest(ctx, "DeleteUser", req, resp)
	return resp, err
}
