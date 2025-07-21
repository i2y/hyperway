package rpc_test

import (
	"testing"

	"github.com/i2y/hyperway/rpc"
)

func TestError(t *testing.T) {
	tests := []struct {
		name           string
		err            *rpc.Error
		expectedCode   rpc.Code
		expectedMsg    string
		expectedString string
	}{
		{
			name:           "basic error",
			err:            rpc.NewError(rpc.CodeInvalidArgument, "invalid input"),
			expectedCode:   rpc.CodeInvalidArgument,
			expectedMsg:    "invalid input",
			expectedString: "invalid_argument: invalid input",
		},
		{
			name:           "error with format",
			err:            rpc.NewErrorf(rpc.CodeNotFound, "user %s not found", "123"),
			expectedCode:   rpc.CodeNotFound,
			expectedMsg:    "user 123 not found",
			expectedString: "not_found: user 123 not found",
		},
		{
			name: "error with details",
			err: rpc.NewError(rpc.CodeFailedPrecondition, "precondition failed").
				WithDetails(map[string]any{"field": "email", "reason": "already exists"}),
			expectedCode:   rpc.CodeFailedPrecondition,
			expectedMsg:    "precondition failed",
			expectedString: "failed_precondition: precondition failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, tt.err.Code)
			}
			if tt.err.Message != tt.expectedMsg {
				t.Errorf("Expected message %s, got %s", tt.expectedMsg, tt.err.Message)
			}
			if tt.err.Error() != tt.expectedString {
				t.Errorf("Expected string %s, got %s", tt.expectedString, tt.err.Error())
			}
		})
	}
}

func TestHTTPStatusCode(t *testing.T) {
	tests := []struct {
		code       rpc.Code
		httpStatus int
	}{
		{rpc.CodeCanceled, 408},           // Request Timeout
		{rpc.CodeUnknown, 500},            // Internal Server Error
		{rpc.CodeInvalidArgument, 400},    // Bad Request
		{rpc.CodeDeadlineExceeded, 408},   // Request Timeout
		{rpc.CodeNotFound, 404},           // Not Found
		{rpc.CodeAlreadyExists, 409},      // Conflict
		{rpc.CodePermissionDenied, 403},   // Forbidden
		{rpc.CodeResourceExhausted, 429},  // Too Many Requests
		{rpc.CodeFailedPrecondition, 412}, // Precondition Failed
		{rpc.CodeAborted, 409},            // Conflict
		{rpc.CodeOutOfRange, 400},         // Bad Request
		{rpc.CodeUnimplemented, 501},      // Not Implemented
		{rpc.CodeInternal, 500},           // Internal Server Error
		{rpc.CodeUnavailable, 503},        // Service Unavailable
		{rpc.CodeDataLoss, 500},           // Internal Server Error
		{rpc.CodeUnauthenticated, 401},    // Unauthorized
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			status := tt.code.HTTPStatusCode()
			if status != tt.httpStatus {
				t.Errorf("Expected HTTP status %d for code %s, got %d", tt.httpStatus, tt.code, status)
			}
		})
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	t.Run("ErrInvalidArgument", func(t *testing.T) {
		err := rpc.ErrInvalidArgument("bad input")
		if err.Code != rpc.CodeInvalidArgument {
			t.Errorf("Expected code %s, got %s", rpc.CodeInvalidArgument, err.Code)
		}
	})

	t.Run("ErrNotFound", func(t *testing.T) {
		err := rpc.ErrNotFound("not found")
		if err.Code != rpc.CodeNotFound {
			t.Errorf("Expected code %s, got %s", rpc.CodeNotFound, err.Code)
		}
	})

	t.Run("ErrInternal", func(t *testing.T) {
		err := rpc.ErrInternal("internal error")
		if err.Code != rpc.CodeInternal {
			t.Errorf("Expected code %s, got %s", rpc.CodeInternal, err.Code)
		}
	})

	t.Run("ErrUnimplemented", func(t *testing.T) {
		err := rpc.ErrUnimplemented("not implemented")
		if err.Code != rpc.CodeUnimplemented {
			t.Errorf("Expected code %s, got %s", rpc.CodeUnimplemented, err.Code)
		}
	})

	t.Run("ErrDeadlineExceeded", func(t *testing.T) {
		err := rpc.ErrDeadlineExceeded("timeout")
		if err.Code != rpc.CodeDeadlineExceeded {
			t.Errorf("Expected code %s, got %s", rpc.CodeDeadlineExceeded, err.Code)
		}
	})

	t.Run("ErrUnauthenticated", func(t *testing.T) {
		err := rpc.ErrUnauthenticated("not authenticated")
		if err.Code != rpc.CodeUnauthenticated {
			t.Errorf("Expected code %s, got %s", rpc.CodeUnauthenticated, err.Code)
		}
	})

	t.Run("ErrPermissionDenied", func(t *testing.T) {
		err := rpc.ErrPermissionDenied("access denied")
		if err.Code != rpc.CodePermissionDenied {
			t.Errorf("Expected code %s, got %s", rpc.CodePermissionDenied, err.Code)
		}
	})
}
