package compatibility

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/i2y/hyperway/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// TestClientStreaming tests client streaming functionality
func TestClientStreaming(t *testing.T) {
	// Create Hyperway server
	handler, err := CreateHyperwayServer()
	require.NoError(t, err)

	// Setup HTTP/2 server
	h2s := &http2.Server{}
	server := httptest.NewServer(h2c.NewHandler(handler, h2s))
	defer server.Close()

	// Create client service
	clientSvc := rpc.NewService("CompatibilityService",
		rpc.WithPackage("compatibility.v1"),
	)

	// Register client streaming method
	err = rpc.RegisterClientStream[ClientStreamRequest, ClientStreamResponse](clientSvc, "ClientStream",
		func(ctx context.Context, stream rpc.ClientStream[ClientStreamRequest]) (*ClientStreamResponse, error) {
			// This is just for type registration on client side
			return nil, nil
		})
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name          string
		values        []int32
		expectedTotal int32
		expectedCount int32
	}{
		{
			name:          "Single value",
			values:        []int32{5},
			expectedTotal: 5,
			expectedCount: 1,
		},
		{
			name:          "Multiple values",
			values:        []int32{1, 2, 3, 4, 5},
			expectedTotal: 15,
			expectedCount: 5,
		},
		{
			name:          "Negative values",
			values:        []int32{-5, 10, -3},
			expectedTotal: 2,
			expectedCount: 3,
		},
		{
			name:          "Empty stream",
			values:        []int32{},
			expectedTotal: 0,
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock client stream implementation
			mockStream := &mockClientStream{
				values: tc.values,
			}

			// Call the handler directly
			ctx := context.Background()
			resp, err := ClientStream(ctx, mockStream)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedTotal, resp.Total)
			assert.Equal(t, tc.expectedCount, resp.Count)
		})
	}
}

// TestBidirectionalStreaming tests bidirectional streaming functionality
func TestBidirectionalStreaming(t *testing.T) {
	// Create Hyperway server
	handler, err := CreateHyperwayServer()
	require.NoError(t, err)

	// Setup HTTP/2 server
	h2s := &http2.Server{}
	server := httptest.NewServer(h2c.NewHandler(handler, h2s))
	defer server.Close()

	// Test cases
	testCases := []struct {
		name     string
		messages []BidiStreamRequest
		validate func(t *testing.T, responses []BidiStreamResponse)
	}{
		{
			name: "Single message",
			messages: []BidiStreamRequest{
				{Message: "Hello", Index: 1},
			},
			validate: func(t *testing.T, responses []BidiStreamResponse) {
				require.Len(t, responses, 1)
				assert.Equal(t, "Echo: Hello", responses[0].Echo)
				assert.Equal(t, int32(1), responses[0].Index)
				assert.Greater(t, responses[0].Timestamp, int64(0))
			},
		},
		{
			name: "Multiple messages",
			messages: []BidiStreamRequest{
				{Message: "First", Index: 1},
				{Message: "Second", Index: 2},
				{Message: "Third", Index: 3},
			},
			validate: func(t *testing.T, responses []BidiStreamResponse) {
				require.Len(t, responses, 3)
				assert.Equal(t, "Echo: First", responses[0].Echo)
				assert.Equal(t, "Echo: Second", responses[1].Echo)
				assert.Equal(t, "Echo: Third", responses[2].Echo)
				assert.Equal(t, int32(1), responses[0].Index)
				assert.Equal(t, int32(2), responses[1].Index)
				assert.Equal(t, int32(3), responses[2].Index)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock bidirectional stream
			mockStream := &mockBidiStream{
				messages:  tc.messages,
				responses: []BidiStreamResponse{},
			}

			// Call the handler
			ctx := context.Background()
			err := BidiStream(ctx, mockStream)
			require.NoError(t, err)

			// Validate responses
			tc.validate(t, mockStream.responses)
		})
	}
}

// TestStreamingWithTimeout tests streaming with timeout
func TestStreamingWithTimeout(t *testing.T) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Mock a slow client stream
	slowStream := &slowClientStream{
		delay: 200 * time.Millisecond,
		ctx:   ctx,
	}

	// This should timeout or return error
	resp, err := ClientStream(ctx, slowStream)
	// The handler doesn't check context, so it might succeed but with only one value
	if err == nil {
		// If no error, should have received at least one value before timeout
		assert.NotNil(t, resp)
		assert.Equal(t, int32(1), resp.Total)
		assert.Equal(t, int32(1), resp.Count)
	}
}

// Mock implementations for testing

type mockClientStream struct {
	values []int32
	index  int
}

func (m *mockClientStream) Recv() (*ClientStreamRequest, error) {
	if m.index >= len(m.values) {
		return nil, io.EOF
	}
	req := &ClientStreamRequest{Value: m.values[m.index]}
	m.index++
	return req, nil
}

func (m *mockClientStream) Context() context.Context {
	return context.Background()
}

type mockBidiStream struct {
	messages  []BidiStreamRequest
	responses []BidiStreamResponse
	index     int
}

func (m *mockBidiStream) Send(resp *BidiStreamResponse) error {
	m.responses = append(m.responses, *resp)
	return nil
}

func (m *mockBidiStream) Recv() (*BidiStreamRequest, error) {
	if m.index >= len(m.messages) {
		return nil, io.EOF
	}
	req := m.messages[m.index]
	m.index++
	return &req, nil
}

func (m *mockBidiStream) Context() context.Context {
	return context.Background()
}

type slowClientStream struct {
	delay  time.Duration
	called bool
	ctx    context.Context
}

func (s *slowClientStream) Recv() (*ClientStreamRequest, error) {
	if s.called {
		return nil, io.EOF
	}
	s.called = true

	// Check if context is already done
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	default:
	}

	// Sleep with context check
	select {
	case <-time.After(s.delay):
		return &ClientStreamRequest{Value: 1}, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *slowClientStream) Context() context.Context {
	return s.ctx
}
