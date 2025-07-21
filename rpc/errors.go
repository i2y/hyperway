// Package rpc provides error codes and handling for Connect/gRPC protocols.
package rpc

import (
	"fmt"
	"net/http"
)

// Code represents a Connect/gRPC error code.
type Code string

// Standard Connect/gRPC error codes.
const (
	CodeCanceled           Code = "canceled"
	CodeUnknown            Code = "unknown"
	CodeInvalidArgument    Code = "invalid_argument"
	CodeDeadlineExceeded   Code = "deadline_exceeded"
	CodeNotFound           Code = "not_found"
	CodeAlreadyExists      Code = "already_exists"
	CodePermissionDenied   Code = "permission_denied"
	CodeResourceExhausted  Code = "resource_exhausted"
	CodeFailedPrecondition Code = "failed_precondition"
	CodeAborted            Code = "aborted"
	CodeOutOfRange         Code = "out_of_range"
	CodeUnimplemented      Code = "unimplemented"
	CodeInternal           Code = "internal"
	CodeUnavailable        Code = "unavailable"
	CodeDataLoss           Code = "data_loss"
	CodeUnauthenticated    Code = "unauthenticated"
)

// Error represents a Connect/gRPC error with code and message.
type Error struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError creates a new Error with the given code and message.
func NewError(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new Error with the given code and formatted message.
func NewErrorf(code Code, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// WithDetails adds details to the error.
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// httpStatusCodeMap maps error codes to HTTP status codes.
var httpStatusCodeMap = map[Code]int{
	CodeCanceled:           http.StatusRequestTimeout,
	CodeUnknown:            http.StatusInternalServerError,
	CodeInvalidArgument:    http.StatusBadRequest,
	CodeDeadlineExceeded:   http.StatusRequestTimeout,
	CodeNotFound:           http.StatusNotFound,
	CodeAlreadyExists:      http.StatusConflict,
	CodePermissionDenied:   http.StatusForbidden,
	CodeResourceExhausted:  http.StatusTooManyRequests,
	CodeFailedPrecondition: http.StatusPreconditionFailed,
	CodeAborted:            http.StatusConflict,
	CodeOutOfRange:         http.StatusBadRequest,
	CodeUnimplemented:      http.StatusNotImplemented,
	CodeInternal:           http.StatusInternalServerError,
	CodeUnavailable:        http.StatusServiceUnavailable,
	CodeDataLoss:           http.StatusInternalServerError,
	CodeUnauthenticated:    http.StatusUnauthorized,
}

// HTTPStatusCode returns the HTTP status code for the error code.
// For Connect protocol, most errors return 200 OK with error in body.
// This is used for non-Connect protocol responses.
func (c Code) HTTPStatusCode() int {
	if status, ok := httpStatusCodeMap[c]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Common error constructors for convenience.

// ErrInvalidArgument creates an invalid argument error.
func ErrInvalidArgument(message string) *Error {
	return NewError(CodeInvalidArgument, message)
}

// ErrNotFound creates a not found error.
func ErrNotFound(message string) *Error {
	return NewError(CodeNotFound, message)
}

// ErrInternal creates an internal error.
func ErrInternal(message string) *Error {
	return NewError(CodeInternal, message)
}

// ErrUnimplemented creates an unimplemented error.
func ErrUnimplemented(message string) *Error {
	return NewError(CodeUnimplemented, message)
}

// ErrDeadlineExceeded creates a deadline exceeded error.
func ErrDeadlineExceeded(message string) *Error {
	return NewError(CodeDeadlineExceeded, message)
}

// ErrUnauthenticated creates an unauthenticated error.
func ErrUnauthenticated(message string) *Error {
	return NewError(CodeUnauthenticated, message)
}

// ErrPermissionDenied creates a permission denied error.
func ErrPermissionDenied(message string) *Error {
	return NewError(CodePermissionDenied, message)
}
