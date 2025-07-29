// Package rpc provides error detail handling for different protocols.
package rpc

import (
	"encoding/base64"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Protocol constants
const (
	protocolConnect = "connect"
	protocolGRPC    = "grpc"
	protocolGRPCWeb = "grpc-web"
)

// ErrorDetail represents a structured error detail.
type ErrorDetail struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// ErrorWithDetails extends Error with structured details.
type ErrorWithDetails struct {
	base    *Error
	details []*ErrorDetail
}

// Error implements the error interface.
func (e *ErrorWithDetails) Error() string {
	return e.base.Error()
}

// Code returns the error code.
func (e *ErrorWithDetails) Code() Code {
	return e.base.Code
}

// Message returns the error message.
func (e *ErrorWithDetails) Message() string {
	return e.base.Message
}

// NewErrorWithDetails creates a new error with details.
func NewErrorWithDetails(code Code, message string, details ...*ErrorDetail) *ErrorWithDetails {
	return &ErrorWithDetails{
		base:    NewError(code, message),
		details: details,
	}
}

// AddDetail adds a detail to the error.
func (e *ErrorWithDetails) AddDetail(detail *ErrorDetail) *ErrorWithDetails {
	e.details = append(e.details, detail)
	return e
}

// AddAnyDetail adds a protobuf Any detail.
func (e *ErrorWithDetails) AddAnyDetail(msg proto.Message) *ErrorWithDetails {
	any, err := anypb.New(msg)
	if err != nil {
		// If we can't create Any, add as regular detail
		e.details = append(e.details, &ErrorDetail{
			Type:  "error",
			Value: err.Error(),
		})
		return e
	}

	// Extract the type name without the URL prefix for Connect protocol
	typeName := any.TypeUrl
	if idx := strings.LastIndex(typeName, "/"); idx >= 0 {
		typeName = typeName[idx+1:]
	}

	e.details = append(e.details, &ErrorDetail{
		Type:  typeName,
		Value: any.Value,
	})
	return e
}

// FormatForProtocol formats error details for the specific protocol.
func (e *ErrorWithDetails) FormatForProtocol(protocol string) any {
	if len(e.details) == 0 {
		return nil
	}

	switch protocol {
	case protocolConnect:
		// Connect expects an array of details with type and value fields
		details := make([]map[string]any, 0, len(e.details))
		for _, d := range e.details {
			detail := map[string]any{
				"type": d.Type,
			}

			// If value is []byte, encode as base64 (unpadded for Connect protocol)
			if b, ok := d.Value.([]byte); ok {
				detail["value"] = base64.RawStdEncoding.EncodeToString(b)
			} else {
				detail["value"] = d.Value
			}

			details = append(details, detail)
		}
		return details

	case protocolGRPC, protocolGRPCWeb:
		// gRPC uses google.rpc.Status with details as Any messages
		// For now, return simplified version
		if len(e.details) > 0 {
			return e.details[0].Value
		}
		return nil

	default:
		// Return raw details
		return e.details
	}
}

// GetDetails returns the error details.
func (e *ErrorWithDetails) GetDetails() []*ErrorDetail {
	return e.details
}

// ToError converts to regular Error with details.
func (e *ErrorWithDetails) ToError(protocol string) *Error {
	err := &Error{
		Code:    e.base.Code,
		Message: e.base.Message,
	}

	if details := e.FormatForProtocol(protocol); details != nil {
		err.Details = map[string]any{
			"details": details,
		}
	}

	return err
}
