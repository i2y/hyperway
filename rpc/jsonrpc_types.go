package rpc

import (
	"encoding/json"
)

// JSON-RPC 2.0 Specification Types

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"` // Can be string, number, or null
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id"` // Must match the request ID
}

// JSONRPCError represents a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON-RPC 2.0 Error Codes
const (
	// Standard JSON-RPC 2.0 error codes
	JSONRPCParseError     = -32700 // Invalid JSON was received by the server
	JSONRPCInvalidRequest = -32600 // The JSON sent is not a valid Request object
	JSONRPCMethodNotFound = -32601 // The method does not exist / is not available
	JSONRPCInvalidParams  = -32602 // Invalid method parameter(s)
	JSONRPCInternalError  = -32603 // Internal JSON-RPC error

	// Server error codes (reserved range: -32000 to -32099)
	JSONRPCServerError = -32000 // Generic server error
)

// IsNotification returns true if this is a notification (no ID)
func (r *JSONRPCRequest) IsNotification() bool {
	return r.ID == nil
}

// IsBatchRequest checks if the raw message is a batch request
func IsBatchRequest(data []byte) bool {
	// A batch request starts with '[' after trimming whitespace
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '['
	}
	return false
}

// errorCodeToJSONRPC maps hyperway error codes to JSON-RPC error codes
func errorCodeToJSONRPC(code Code) int {
	switch code {
	case CodeInvalidArgument:
		return JSONRPCInvalidParams
	case CodeNotFound:
		return JSONRPCMethodNotFound
	case CodeInternal:
		return JSONRPCInternalError
	case CodeUnimplemented:
		return JSONRPCMethodNotFound
	case CodeCanceled,
		CodeUnknown,
		CodeDeadlineExceeded,
		CodeAlreadyExists,
		CodePermissionDenied,
		CodeResourceExhausted,
		CodeFailedPrecondition,
		CodeAborted,
		CodeOutOfRange,
		CodeUnavailable,
		CodeDataLoss,
		CodeUnauthenticated:
		return JSONRPCServerError
	default:
		return JSONRPCServerError
	}
}

// NewJSONRPCError creates a JSON-RPC error from a hyperway error
func NewJSONRPCError(err *Error) *JSONRPCError {
	return &JSONRPCError{
		Code:    errorCodeToJSONRPC(err.Code),
		Message: err.Message,
		Data:    err.Details,
	}
}
