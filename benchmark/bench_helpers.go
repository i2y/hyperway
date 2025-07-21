package benchmark

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fastHTTPClient is an optimized HTTP client for benchmarks
type fastHTTPClient struct {
	client *http.Client
}

func newFastHTTPClient() *fastHTTPClient {
	return &fastHTTPClient{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
				MaxConnsPerHost:     100,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // for benchmarks only
			},
		},
	}
}

func (c *fastHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// startTestServer starts an HTTP test server with HTTP/2 support
func startTestServer(b *testing.B, handler http.Handler) *httptest.Server {
	b.Helper()
	server := httptest.NewUnstartedServer(handler)
	// Enable HTTP/2
	server.EnableHTTP2 = true
	server.StartTLS()
	return server
}

// Connect client adapter
func (c *fastHTTPClient) CallUnary(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.Do(req)
}

// Simple Connect-style request/response for benchmarks
type connectRequest[T any] struct {
	Msg T
}

func newConnectRequest[T any](msg T) *connectRequest[T] {
	return &connectRequest[T]{Msg: msg}
}

type connectResponse[T any] struct {
	Msg T
}

// Simple Connect client implementation for benchmarks
type connectClient[Req, Res any] struct {
	httpClient *fastHTTPClient
	url        string
}

func newConnectClient[Req, Res any](url string) *connectClient[Req, Res] {
	return &connectClient[Req, Res]{
		httpClient: newFastHTTPClient(),
		url:        url,
	}
}

func (c *connectClient[Req, Res]) CallUnary(ctx context.Context, req *connectRequest[Req]) (*connectResponse[Res], error) {
	// Simplified Connect protocol implementation for benchmarks
	// In reality, this would use the full Connect protocol

	// For benchmark purposes, we'll make a direct HTTP call
	body, err := json.Marshal(req.Msg)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var resMsg Res
	if err := json.Unmarshal(respBody, &resMsg); err != nil {
		return nil, err
	}

	return &connectResponse[Res]{Msg: resMsg}, nil
}
