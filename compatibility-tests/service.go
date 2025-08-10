// Package compatibility provides test types and handlers for compatibility testing
// between Hyperway and Connect-go.
package compatibility

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/i2y/hyperway/rpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Test message types for various data type compatibility tests

// SimpleRequest tests basic scalar types
type SimpleRequest struct {
	StringField string  `json:"string_field"`
	Int32Field  int32   `json:"int32_field"`
	Int64Field  int64   `json:"int64_field"`
	FloatField  float32 `json:"float_field"`
	DoubleField float64 `json:"double_field"`
	BoolField   bool    `json:"bool_field"`
	BytesField  []byte  `json:"bytes_field"`
}

// SimpleResponse mirrors the request for echo testing
type SimpleResponse struct {
	StringField string  `json:"string_field"`
	Int32Field  int32   `json:"int32_field"`
	Int64Field  int64   `json:"int64_field"`
	FloatField  float32 `json:"float_field"`
	DoubleField float64 `json:"double_field"`
	BoolField   bool    `json:"bool_field"`
	BytesField  []byte  `json:"bytes_field"`
}

// ComplexRequest tests nested messages, repeated fields, and maps
type ComplexRequest struct {
	NestedMessage *NestedMessage   `json:"nested_message"`
	RepeatedField []string         `json:"repeated_field"`
	MapField      map[string]int32 `json:"map_field"`
	OptionalField *string          `json:"optional_field,omitempty"`
	OneofField    *OneofMessage    `json:"oneof_field,omitempty"`
}

// ComplexResponse mirrors the complex request
type ComplexResponse struct {
	NestedMessage *NestedMessage   `json:"nested_message"`
	RepeatedField []string         `json:"repeated_field"`
	MapField      map[string]int32 `json:"map_field"`
	OptionalField *string          `json:"optional_field,omitempty"`
	OneofField    *OneofMessage    `json:"oneof_field,omitempty"`
}

// NestedMessage tests nested message handling
type NestedMessage struct {
	ID    string `json:"id"`
	Value int32  `json:"value"`
}

// OneofMessage tests oneof field handling
type OneofMessage struct {
	// Oneof fields - only one should be set
	Choice struct {
		StringChoice *string `json:"string_choice,omitempty"`
		IntChoice    *int32  `json:"int_choice,omitempty"`
		BoolChoice   *bool   `json:"bool_choice,omitempty"`
	} `hyperway:"oneof"`
}

// WellKnownRequest tests Well-Known Types
type WellKnownRequest struct {
	Timestamp   *timestamppb.Timestamp  `json:"timestamp"`
	Duration    *durationpb.Duration    `json:"duration"`
	StringValue *wrapperspb.StringValue `json:"string_value"`
	Int32Value  *wrapperspb.Int32Value  `json:"int32_value"`
	BoolValue   *wrapperspb.BoolValue   `json:"bool_value"`
	EmptyValue  *emptypb.Empty          `json:"empty_value"`
}

// WellKnownResponse mirrors well-known types request
type WellKnownResponse struct {
	Timestamp   *timestamppb.Timestamp  `json:"timestamp"`
	Duration    *durationpb.Duration    `json:"duration"`
	StringValue *wrapperspb.StringValue `json:"string_value"`
	Int32Value  *wrapperspb.Int32Value  `json:"int32_value"`
	BoolValue   *wrapperspb.BoolValue   `json:"bool_value"`
	EmptyValue  *emptypb.Empty          `json:"empty_value"`
}

// StreamRequest for streaming tests
type StreamRequest struct {
	Count int32 `json:"count"`
}

// StreamResponse for streaming tests
type StreamResponse struct {
	Index   int32  `json:"index"`
	Message string `json:"message"`
}

// ErrorRequest for error handling tests
type ErrorRequest struct {
	ErrorCode string `json:"error_code"`
}

// ErrorResponse for error handling tests
type ErrorResponse struct {
	Success bool `json:"success"`
}

// Service Handlers

// SimpleEcho echoes back the simple request
func SimpleEcho(ctx context.Context, req *SimpleRequest) (*SimpleResponse, error) {
	return &SimpleResponse{
		StringField: req.StringField,
		Int32Field:  req.Int32Field,
		Int64Field:  req.Int64Field,
		FloatField:  req.FloatField,
		DoubleField: req.DoubleField,
		BoolField:   req.BoolField,
		BytesField:  req.BytesField,
	}, nil
}

// ComplexEcho echoes back the complex request
func ComplexEcho(ctx context.Context, req *ComplexRequest) (*ComplexResponse, error) {
	return &ComplexResponse{
		NestedMessage: req.NestedMessage,
		RepeatedField: req.RepeatedField,
		MapField:      req.MapField,
		OptionalField: req.OptionalField,
		OneofField:    req.OneofField,
	}, nil
}

// WellKnownEcho echoes back well-known types
func WellKnownEcho(ctx context.Context, req *WellKnownRequest) (*WellKnownResponse, error) {
	return &WellKnownResponse{
		Timestamp:   req.Timestamp,
		Duration:    req.Duration,
		StringValue: req.StringValue,
		Int32Value:  req.Int32Value,
		BoolValue:   req.BoolValue,
		EmptyValue:  req.EmptyValue,
	}, nil
}

// ServerStreamInterface defines the interface for server streaming
type ServerStreamInterface interface {
	Send(*StreamResponse) error
	Context() context.Context
}

// ServerStream streams responses to the client
func ServerStream(ctx context.Context, req *StreamRequest, stream ServerStreamInterface) error {
	for i := int32(0); i < req.Count; i++ {
		resp := &StreamResponse{
			Index:   i,
			Message: fmt.Sprintf("Stream message %d", i),
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
		// Small delay to simulate processing
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// ServerStreamLarge streams large responses to test compression
func ServerStreamLarge(ctx context.Context, req *StreamRequest, stream ServerStreamInterface) error {
	// Generate a large base message that will trigger compression (>1KB)
	largeMessage := strings.Repeat("This is a large streaming message for compression testing! ", 50)

	for i := int32(0); i < req.Count; i++ {
		resp := &StreamResponse{
			Index:   i,
			Message: fmt.Sprintf("%s [Message #%d]", largeMessage, i),
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
		// Small delay to simulate processing
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// TestError returns various error codes for testing
func TestError(ctx context.Context, req *ErrorRequest) (*ErrorResponse, error) {
	switch req.ErrorCode {
	case "not_found":
		return nil, rpc.NewError(rpc.CodeNotFound, "not found")
	case "invalid_argument":
		return nil, rpc.NewError(rpc.CodeInvalidArgument, "invalid argument")
	case "internal":
		return nil, rpc.NewError(rpc.CodeInternal, "internal server error")
	case "unauthenticated":
		return nil, rpc.NewError(rpc.CodeUnauthenticated, "unauthenticated")
	case "permission_denied":
		return nil, rpc.NewError(rpc.CodePermissionDenied, "permission denied")
	case "":
		// No error
		return &ErrorResponse{Success: true}, nil
	default:
		return nil, rpc.NewErrorf(rpc.CodeUnknown, "unknown error code: %s", req.ErrorCode)
	}
}
