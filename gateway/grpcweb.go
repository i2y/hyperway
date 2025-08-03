package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Constants
const (
	defaultRequestTimeout = 30 * time.Second
)

// HTTP header constants
const (
	headerContentType = "content-type"
)

// grpcWebHandler handles gRPC-Web requests
type grpcWebHandler struct {
	// The underlying gRPC handler
	grpcHandler http.Handler
	// Timeout for requests
	timeout time.Duration
}

// newGRPCWebHandler creates a new gRPC-Web handler
func newGRPCWebHandler(grpcHandler http.Handler, timeout time.Duration) *grpcWebHandler {
	if timeout == 0 {
		timeout = defaultRequestTimeout
	}
	return &grpcWebHandler{
		grpcHandler: grpcHandler,
		timeout:     timeout,
	}
}

// ServeHTTP implements http.Handler for gRPC-Web
func (h *grpcWebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create codec based on content type
	codec := newGRPCWebCodec(r.Header.Get("Content-Type"))

	// Set response content type
	w.Header().Set("Content-Type", codec.contentType())

	// Create frame reader and writer
	frameReader := newGRPCWebFrameReader(r.Body, codec.mode)
	frameWriter := newGRPCWebFrameWriter(w, codec.mode)
	defer func() {
		_ = frameWriter.close()
	}()

	// Process the request
	h.processRequest(w, r, frameReader, frameWriter, codec)
}

// processRequest handles the main request processing logic
func (h *grpcWebHandler) processRequest(w http.ResponseWriter, r *http.Request, frameReader *grpcWebFrameReader, frameWriter *grpcWebFrameWriter, codec *grpcWebCodec) {
	// Read the request message
	requestData, err := h.readRequestMessage(frameReader)
	if err != nil {
		h.writeErrorResponse(frameWriter, err)
		return
	}

	// Create a new request for the underlying gRPC handler
	grpcReq, err := h.createGRPCRequest(r, requestData, codec)
	if err != nil {
		h.writeErrorResponse(frameWriter, err)
		return
	}

	// Create a response recorder to capture the gRPC response
	recorder := newResponseRecorder()

	// Call the underlying gRPC handler
	h.grpcHandler.ServeHTTP(recorder, grpcReq)

	// Handle the response
	h.handleResponse(w, frameWriter, recorder, codec)
}

// handleResponse processes the response from the gRPC handler
func (h *grpcWebHandler) handleResponse(w http.ResponseWriter, frameWriter *grpcWebFrameWriter, recorder *responseRecorder, codec *grpcWebCodec) {
	// Ensure we write 200 OK for gRPC-Web
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}

	// Check if the handler returned 404 (not found)
	if recorder.status == http.StatusNotFound {
		h.writeUnimplementedError(frameWriter)
		return
	}

	// Check if the handler returned an error via grpc-status header
	if grpcStatus := recorder.Header().Get("grpc-status"); grpcStatus != "" && grpcStatus != "0" {
		h.writeResponseWithError(frameWriter, recorder)
		return
	}

	// For JSON responses, check if the body contains an error
	if codec.isJSON && recorder.body.Len() > 0 {
		if h.handleJSONError(frameWriter, recorder.body.Bytes()) {
			return
		}
	}

	// Write HTTP status before frames
	w.WriteHeader(http.StatusOK)

	// Write the response
	if err := h.writeResponse(frameWriter, recorder); err != nil {
		// If we fail to write response, there's not much we can do
		// as we may have already started writing to the client
		return
	}

	// Ensure response is flushed
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// handleJSONError checks for and handles JSON error responses
func (h *grpcWebHandler) handleJSONError(frameWriter *grpcWebFrameWriter, bodyBytes []byte) bool {
	// Try to detect Connect-style JSON error response
	if !bytes.Contains(bodyBytes, []byte(`"error"`)) && !bytes.Contains(bodyBytes, []byte(`"code"`)) {
		return false
	}

	var errorResp struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &errorResp); err != nil || (errorResp.Error == "" && errorResp.Code == "") {
		return false
	}

	// Convert to gRPC-Web error
	code := h.parseErrorCode(errorResp.Code, errorResp.Error, errorResp.Message)
	message := errorResp.Error
	if message == "" {
		message = errorResp.Message
	}

	h.writeErrorStatus(frameWriter, code, message)
	return true
}

// parseErrorCode converts string error codes to gRPC codes
func (h *grpcWebHandler) parseErrorCode(codeStr, errorMsg, message string) codes.Code {
	if code, ok := stringToGRPCCode[codeStr]; ok {
		return code
	}

	// If there's no code but we have an error message, try to infer from message
	if codeStr == "" && (errorMsg != "" || message != "") {
		msg := errorMsg
		if msg == "" {
			msg = message
		}
		if strings.Contains(strings.ToLower(msg), "unimplemented") {
			return codes.Unimplemented
		}
	}

	return codes.Unknown
}

// stringToGRPCCode maps string codes to gRPC codes
var stringToGRPCCode = map[string]codes.Code{
	"canceled":            codes.Canceled,
	"unknown":             codes.Unknown,
	"invalid_argument":    codes.InvalidArgument,
	"deadline_exceeded":   codes.DeadlineExceeded,
	"not_found":           codes.NotFound,
	"already_exists":      codes.AlreadyExists,
	"permission_denied":   codes.PermissionDenied,
	"resource_exhausted":  codes.ResourceExhausted,
	"failed_precondition": codes.FailedPrecondition,
	"aborted":             codes.Aborted,
	"out_of_range":        codes.OutOfRange,
	"unimplemented":       codes.Unimplemented,
	"internal":            codes.Internal,
	"unavailable":         codes.Unavailable,
	"data_loss":           codes.DataLoss,
	"unauthenticated":     codes.Unauthenticated,
}

// readRequestMessage reads the request message from gRPC-Web frames
func (h *grpcWebHandler) readRequestMessage(reader *grpcWebFrameReader) ([]byte, error) {
	var requestData []byte

	for {
		frame, err := reader.readFrame()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, status.Errorf(codes.InvalidArgument, "failed to read frame: %v", err)
		}

		// We only expect data frames in the request
		if frame.isTrailer() {
			return nil, status.Error(codes.InvalidArgument, "unexpected trailer frame in request")
		}

		requestData = append(requestData, frame.payload...)
	}

	return requestData, nil
}

// createGRPCRequest creates a new HTTP request for the underlying gRPC handler
func (h *grpcWebHandler) createGRPCRequest(originalReq *http.Request, requestData []byte, codec *grpcWebCodec) (*http.Request, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(originalReq.Context(), h.timeout)
	// Don't defer cancel here, let the caller handle it
	_ = cancel

	// Create new request with raw protobuf data (no gRPC frame)
	// Hyperway expects raw protobuf, not gRPC frames
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, originalReq.URL.String(), bytes.NewReader(requestData))
	if err != nil {
		return nil, err
	}

	// Copy relevant headers
	req.Header = make(http.Header)
	for key, values := range originalReq.Header {
		// Skip gRPC-Web specific headers
		if strings.HasPrefix(strings.ToLower(key), "x-grpc-web") {
			continue
		}
		// Convert certain headers
		switch strings.ToLower(key) {
		case headerContentType:
			// Convert gRPC-Web content type to standard format for handler
			if codec.isJSON {
				req.Header.Set("Content-Type", "application/json")
			} else {
				req.Header.Set("Content-Type", "application/protobuf")
			}
			// Also set a flag to indicate gRPC-Web
			req.Header.Set("X-Grpc-Web", "1")
		case "x-user-agent":
			// Convert X-User-Agent to User-Agent for gRPC
			req.Header.Set("User-Agent", values[0])
		default:
			req.Header[key] = values
		}
	}

	// Set content length
	req.ContentLength = int64(len(requestData))

	return req, nil
}

// writeResponse writes the gRPC response as gRPC-Web frames
func (h *grpcWebHandler) writeResponse(writer *grpcWebFrameWriter, recorder *responseRecorder) error {
	// Always write a data frame (even if empty) as per gRPC-Web spec
	bodyBytes := recorder.body.Bytes()
	if err := writer.writeDataFrame(bodyBytes); err != nil {
		return err
	}

	// Prepare and write trailers
	trailers := h.prepareTrailers(recorder)
	trailerData := formatTrailerFrame(trailers)
	return writer.writeTrailerFrame(trailerData)
}

// prepareTrailers prepares the trailer headers from the response recorder
func (h *grpcWebHandler) prepareTrailers(recorder *responseRecorder) http.Header {
	trailers := make(http.Header)

	// Extract and set gRPC status
	statusCode, statusMsg := h.extractGRPCStatus(recorder)
	trailers.Set("grpc-status", strconv.Itoa(int(statusCode)))
	if statusMsg != "" {
		trailers.Set("grpc-message", statusMsg)
	}

	// Copy headers to trailers
	h.copyHeadersToTrailers(recorder.Header(), trailers)

	return trailers
}

// extractGRPCStatus extracts gRPC status code and message from response headers
func (h *grpcWebHandler) extractGRPCStatus(recorder *responseRecorder) (statusCode codes.Code, statusMsg string) {
	statusCode = codes.OK
	statusMsg = ""

	// Check if there's a grpc-status header
	if grpcStatus := recorder.Header().Get("grpc-status"); grpcStatus != "" {
		if code, err := strconv.Atoi(grpcStatus); err == nil && code >= 0 && code <= 16 {
			statusCode = codes.Code(code) //nolint:gosec // code range is validated above
		}
	}

	if grpcMessage := recorder.Header().Get("grpc-message"); grpcMessage != "" {
		statusMsg = grpcMessage
	}

	return statusCode, statusMsg
}

// copyHeadersToTrailers copies appropriate headers to trailers
func (h *grpcWebHandler) copyHeadersToTrailers(headers, trailers http.Header) {
	// Copy grpc-prefixed headers (except status and message)
	for key, values := range headers {
		if h.shouldCopyAsGRPCTrailer(key) {
			trailers[key] = values
		}
	}

	// Copy custom headers as trailers
	for key, values := range headers {
		if h.shouldCopyAsCustomTrailer(key) {
			for _, value := range values {
				trailers.Add(key, value)
			}
		}
	}
}

// shouldCopyAsGRPCTrailer checks if a header should be copied as a gRPC trailer
func (h *grpcWebHandler) shouldCopyAsGRPCTrailer(key string) bool {
	lowerKey := strings.ToLower(key)
	return strings.HasPrefix(lowerKey, "grpc-") &&
		key != "grpc-status" &&
		key != "grpc-message"
}

// shouldCopyAsCustomTrailer checks if a header should be copied as a custom trailer
func (h *grpcWebHandler) shouldCopyAsCustomTrailer(key string) bool {
	lowerKey := strings.ToLower(key)

	// Skip grpc-specific headers
	if strings.HasPrefix(lowerKey, "grpc-") {
		return false
	}

	// Skip standard HTTP headers
	standardHeaders := map[string]bool{
		headerContentType: true,
		"content-length":  true,
		"date":            true,
		"server":          true,
	}

	return !standardHeaders[lowerKey]
}

// writeErrorResponse writes an error response
func (h *grpcWebHandler) writeErrorResponse(writer *grpcWebFrameWriter, err error) {
	// Convert error to gRPC status
	st, ok := status.FromError(err)
	if !ok {
		st = status.New(codes.Internal, err.Error())
	}

	// Create trailers with error status
	trailers := make(http.Header)
	trailers.Set("grpc-status", strconv.Itoa(int(st.Code())))
	trailers.Set("grpc-message", st.Message())

	// Write empty data frame followed by trailer
	_ = writer.writeDataFrame(nil)
	_ = writer.writeTrailerFrame(formatTrailerFrame(trailers))
}

// writeUnimplementedError writes an unimplemented error response
func (h *grpcWebHandler) writeUnimplementedError(writer *grpcWebFrameWriter) {
	// Create trailers with unimplemented status
	trailers := make(http.Header)
	trailers.Set("grpc-status", strconv.Itoa(int(codes.Unimplemented)))
	trailers.Set("grpc-message", "Method not found")

	// Write empty data frame followed by trailer
	_ = writer.writeDataFrame(nil)
	_ = writer.writeTrailerFrame(formatTrailerFrame(trailers))
}

// writeErrorStatus writes an error response with specific status code
func (h *grpcWebHandler) writeErrorStatus(writer *grpcWebFrameWriter, code codes.Code, message string) {
	// Create trailers with error status
	trailers := make(http.Header)
	trailers.Set("grpc-status", strconv.Itoa(int(code)))
	trailers.Set("grpc-message", message)

	// Write empty data frame followed by trailer
	_ = writer.writeDataFrame(nil)
	_ = writer.writeTrailerFrame(formatTrailerFrame(trailers))
}

// writeResponseWithError writes a response when we know there's an error in the headers
func (h *grpcWebHandler) writeResponseWithError(writer *grpcWebFrameWriter, recorder *responseRecorder) {
	// Extract status from headers
	statusCode := codes.Unknown
	statusMsg := ""

	if grpcStatus := recorder.Header().Get("grpc-status"); grpcStatus != "" {
		if code, err := strconv.Atoi(grpcStatus); err == nil && code >= 0 && code <= 16 {
			statusCode = codes.Code(code) //nolint:gosec // code range is validated above
		}
	}

	if grpcMessage := recorder.Header().Get("grpc-message"); grpcMessage != "" {
		statusMsg = grpcMessage
	}

	// Write empty data frame and trailers
	h.writeErrorStatus(writer, statusCode, statusMsg)
}

// responseRecorder captures the response from the gRPC handler
type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
		status: http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

// isGRPCWeb checks if the request is a gRPC-Web request
func isGRPCWeb(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	grpcWebHeader := r.Header.Get("X-Grpc-Web") == "1"
	hasGrpcWebContentType := strings.HasPrefix(contentType, "application/grpc-web")

	return hasGrpcWebContentType || grpcWebHeader
}

// extractMetadata extracts gRPC metadata from HTTP headers
func extractMetadata(headers http.Header) metadata.MD {
	md := metadata.MD{}

	for key, values := range headers {
		// Skip non-metadata headers
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-grpc-web") ||
			lowerKey == headerContentType ||
			lowerKey == "user-agent" ||
			lowerKey == "content-length" {
			continue
		}

		// Add to metadata
		md[lowerKey] = values
	}

	return md
}

// grpcWebInterceptor can be used to intercept gRPC-Web requests
type grpcWebInterceptor struct {
	handler http.Handler
	timeout time.Duration
}

// NewGRPCWebInterceptor creates a new gRPC-Web interceptor
func NewGRPCWebInterceptor(handler http.Handler, timeout time.Duration) http.Handler {
	return &grpcWebInterceptor{
		handler: handler,
		timeout: timeout,
	}
}

// ServeHTTP implements http.Handler
func (i *grpcWebInterceptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a gRPC-Web request
	if isGRPCWeb(r) {
		// Handle as gRPC-Web
		webHandler := newGRPCWebHandler(i.handler, i.timeout)
		webHandler.ServeHTTP(w, r)
		return
	}

	// Otherwise, pass through to the original handler
	i.handler.ServeHTTP(w, r)
}
