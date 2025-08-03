package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

// handleJSONRPCRequest handles JSON-RPC 2.0 requests
func (s *Service) handleJSONRPCRequest(w http.ResponseWriter, r *http.Request, _ *handlerContext) {
	// Only accept POST
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, &JSONRPCError{
			Code:    JSONRPCInvalidRequest,
			Message: "Only POST method is allowed",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeJSONRPCError(w, nil, &JSONRPCError{
			Code:    JSONRPCParseError,
			Message: "Failed to read request body",
		})
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Check if it's a batch request
	if IsBatchRequest(body) {
		s.handleJSONRPCBatch(w, r, body)
		return
	}

	// Parse single request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSONRPCError(w, nil, &JSONRPCError{
			Code:    JSONRPCParseError,
			Message: "Invalid JSON",
		})
		return
	}

	// Validate request
	if req.JSONRPC != "2.0" {
		s.writeJSONRPCError(w, req.ID, &JSONRPCError{
			Code:    JSONRPCInvalidRequest,
			Message: "Invalid jsonrpc version",
		})
		return
	}

	// Process the request
	response := s.processJSONRPCRequest(r.Context(), &req)

	// Don't send response for notifications
	if req.IsNotification() && response.Error == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Send response
	s.writeJSONRPCResponse(w, response)
}

// processJSONRPCRequest processes a single JSON-RPC request
func (s *Service) processJSONRPCRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	// Create response with matching ID
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	// Resolve method name
	methodName := s.resolveJSONRPCMethod(req.Method)
	method, exists := s.methods[methodName]
	if !exists {
		resp.Error = &JSONRPCError{
			Code:    JSONRPCMethodNotFound,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
		return resp
	}

	// Check if we have a cached handler context
	cachedCtx, ok := s.handlerCtxCache[method.Name]
	if !ok {
		// Prepare handler context if not cached
		var err error
		cachedCtx, err = s.prepareHandlerContext(method)
		if err != nil {
			resp.Error = &JSONRPCError{
				Code:    JSONRPCInternalError,
				Message: fmt.Sprintf("Failed to prepare handler: %v", err),
			}
			return resp
		}
		// Cache it
		s.handlerCtxCache[method.Name] = cachedCtx
	}

	// Create a new handler context for this request
	handlerCtx := &handlerContext{
		method:           method,
		options:          s.options,
		validator:        s.validator,
		responseHeaders:  make(map[string][]string),
		responseTrailers: make(map[string][]string),
		inputCodec:       cachedCtx.inputCodec,
		outputCodec:      cachedCtx.outputCodec,
		handlerFunc:      cachedCtx.handlerFunc,
		interceptors:     cachedCtx.interceptors,
		useProtoInput:    cachedCtx.useProtoInput,
		useProtoOutput:   cachedCtx.useProtoOutput,
	}

	// Decode parameters
	inputPtr, err := s.decodeJSONRPCParams(req.Params, handlerCtx)
	if err != nil {
		resp.Error = &JSONRPCError{
			Code:    JSONRPCInvalidParams,
			Message: err.Error(),
		}
		return resp
	}

	// Validate input if enabled
	if err := s.validateInput(inputPtr, handlerCtx); err != nil {
		resp.Error = &JSONRPCError{
			Code:    JSONRPCInvalidParams,
			Message: err.Error(),
		}
		return resp
	}

	// Add handler context to the request context
	ctx = context.WithValue(ctx, handlerContextKey, handlerCtx)

	// Call the handler
	output, err := s.callHandler(ctx, inputPtr, handlerCtx)
	if err != nil {
		// Convert to JSON-RPC error
		if rpcErr, ok := err.(*Error); ok {
			resp.Error = NewJSONRPCError(rpcErr)
		} else {
			resp.Error = &JSONRPCError{
				Code:    JSONRPCInternalError,
				Message: err.Error(),
			}
		}
		return resp
	}

	// Encode the result
	resultData, err := json.Marshal(output)
	if err != nil {
		resp.Error = &JSONRPCError{
			Code:    JSONRPCInternalError,
			Message: "Failed to encode response",
		}
		return resp
	}

	resp.Result = resultData
	return resp
}

// resolveJSONRPCMethod converts JSON-RPC method name to internal format
func (s *Service) resolveJSONRPCMethod(method string) string {
	// If method contains dots, it might be fully qualified
	if strings.Contains(method, ".") {
		// Try to convert from JSON-RPC format to gRPC format
		// e.g., "user.v1.UserService.CreateUser" -> "CreateUser"
		parts := strings.Split(method, ".")
		if len(parts) > 0 {
			// Return the last part as the method name
			return parts[len(parts)-1]
		}
	}

	// Return as-is if it's already a simple method name
	return method
}

// decodeJSONRPCParams decodes JSON-RPC parameters into the expected input type
func (s *Service) decodeJSONRPCParams(params json.RawMessage, ctx *handlerContext) (reflect.Value, error) {
	inputType := ctx.method.InputType
	inputPtr := reflect.New(inputType)

	// If params is null or empty, keep the zero value
	if len(params) == 0 || string(params) == "null" {
		return inputPtr, nil
	}

	// Unmarshal params into the input type
	if err := json.Unmarshal(params, inputPtr.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to decode parameters: %w", err)
	}

	return inputPtr, nil
}

// handleJSONRPCBatch handles batch JSON-RPC requests
func (s *Service) handleJSONRPCBatch(w http.ResponseWriter, r *http.Request, body []byte) {
	var requests []JSONRPCRequest
	if err := json.Unmarshal(body, &requests); err != nil {
		s.writeJSONRPCError(w, nil, &JSONRPCError{
			Code:    JSONRPCParseError,
			Message: "Invalid batch request",
		})
		return
	}

	// Check batch size limit
	if len(requests) > s.options.JSONRPCBatchLimit {
		s.writeJSONRPCError(w, nil, &JSONRPCError{
			Code:    JSONRPCInvalidRequest,
			Message: fmt.Sprintf("Batch request exceeds limit of %d", s.options.JSONRPCBatchLimit),
		})
		return
	}

	// Process requests in parallel with a semaphore to limit concurrency
	const maxConcurrency = 10
	sem := make(chan struct{}, maxConcurrency)

	responses := make([]*JSONRPCResponse, 0, len(requests))
	responseMu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for i := range requests {
		req := &requests[i]

		// Validate each request
		if req.JSONRPC != "2.0" {
			responseMu.Lock()
			responses = append(responses, &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    JSONRPCInvalidRequest,
					Message: "Invalid jsonrpc version",
				},
			})
			responseMu.Unlock()
			continue
		}

		// Skip notifications in batch
		if req.IsNotification() {
			continue
		}

		wg.Add(1)
		go func(req *JSONRPCRequest) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Process request
			resp := s.processJSONRPCRequest(r.Context(), req)

			// Add to responses
			responseMu.Lock()
			responses = append(responses, resp)
			responseMu.Unlock()
		}(req)
	}

	wg.Wait()

	// If all requests were notifications, return no content
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Write batch response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		// Log error, but response is already partially written
		log.Printf("Failed to write batch response: %v", err)
	}
}

// writeJSONRPCResponse writes a JSON-RPC response
func (s *Service) writeJSONRPCResponse(w http.ResponseWriter, resp *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Log error, but response is already partially written
		log.Printf("Failed to write JSON-RPC response: %v", err)
	}
}

// writeJSONRPCError writes a JSON-RPC error response
func (s *Service) writeJSONRPCError(w http.ResponseWriter, id interface{}, err *JSONRPCError) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}

	s.writeJSONRPCResponse(w, resp)
}
