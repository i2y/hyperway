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
		timeout = 30 * time.Second
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

	// Don't write status code here - let the handler decide

	// Create frame reader and writer
	frameReader := newGRPCWebFrameReader(r.Body, codec.mode)
	frameWriter := newGRPCWebFrameWriter(w, codec.mode)
	defer func() {
		_ = frameWriter.close()
	}()

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

	// Ensure we write 200 OK for gRPC-Web
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}

	// Check if the handler returned 404 (not found)
	if recorder.status == http.StatusNotFound {
		// Convert to gRPC-Web unimplemented error
		h.writeUnimplementedError(frameWriter)
		return
	}

	// Check if the handler returned an error via grpc-status header
	if grpcStatus := recorder.Header().Get("grpc-status"); grpcStatus != "" && grpcStatus != "0" {
		// This is an error response - handle it specially
		h.writeResponseWithError(frameWriter, recorder)
		return
	}

	// For JSON responses, check if the body contains an error
	if codec.isJSON && recorder.body.Len() > 0 {
		// Try to detect Connect-style JSON error response
		bodyBytes := recorder.body.Bytes()
		if bytes.Contains(bodyBytes, []byte(`"error"`)) || bytes.Contains(bodyBytes, []byte(`"code"`)) {
			// This looks like an error response - convert to gRPC-Web error
			var errorResp struct {
				Error   string `json:"error"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(bodyBytes, &errorResp); err == nil && (errorResp.Error != "" || errorResp.Code != "") {
				// Convert to gRPC-Web error
				code := codes.Unknown
				message := errorResp.Error
				if message == "" {
					message = errorResp.Message
				}

				// Try to parse code
				switch errorResp.Code {
				case "canceled":
					code = codes.Canceled
				case "unknown":
					code = codes.Unknown
				case "invalid_argument":
					code = codes.InvalidArgument
				case "deadline_exceeded":
					code = codes.DeadlineExceeded
				case "not_found":
					code = codes.NotFound
				case "already_exists":
					code = codes.AlreadyExists
				case "permission_denied":
					code = codes.PermissionDenied
				case "resource_exhausted":
					code = codes.ResourceExhausted
				case "failed_precondition":
					code = codes.FailedPrecondition
				case "aborted":
					code = codes.Aborted
				case "out_of_range":
					code = codes.OutOfRange
				case "unimplemented":
					code = codes.Unimplemented
				case "internal":
					code = codes.Internal
				case "unavailable":
					code = codes.Unavailable
				case "data_loss":
					code = codes.DataLoss
				case "unauthenticated":
					code = codes.Unauthenticated
				default:
					// If there's no code but we have an error message, try to infer from message
					if errorResp.Code == "" && message != "" {
						if strings.Contains(strings.ToLower(message), "unimplemented") {
							code = codes.Unimplemented
						}
					}
				}

				h.writeErrorStatus(frameWriter, code, message)
				return
			}
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

	// Prepare trailers
	trailers := make(http.Header)

	// Add gRPC status
	statusCode := codes.OK
	statusMsg := ""

	// Check if there's a grpc-status header
	if grpcStatus := recorder.Header().Get("grpc-status"); grpcStatus != "" {
		if code, err := strconv.Atoi(grpcStatus); err == nil && code >= 0 && code <= 16 {
			statusCode = codes.Code(code) //nolint:gosec // code range is validated above //nolint:gosec // code range is validated above
		}
	}

	if grpcMessage := recorder.Header().Get("grpc-message"); grpcMessage != "" {
		statusMsg = grpcMessage
	}

	// Set status in trailers
	trailers.Set("grpc-status", strconv.Itoa(int(statusCode)))
	if statusMsg != "" {
		trailers.Set("grpc-message", statusMsg)
	}

	// Copy any other trailer headers
	for key, values := range recorder.Header() {
		if strings.HasPrefix(strings.ToLower(key), "grpc-") &&
			key != "grpc-status" && key != "grpc-message" {
			trailers[key] = values
		}
	}

	// Also copy response headers/trailers from recorder
	for key, values := range recorder.Header() {
		lowerKey := strings.ToLower(key)
		// Skip grpc-specific headers that go in trailers
		if strings.HasPrefix(lowerKey, "grpc-") {
			continue
		}
		// Skip standard HTTP headers
		if lowerKey == headerContentType || lowerKey == "content-length" ||
			lowerKey == "date" || lowerKey == "server" {
			continue
		}
		// Add custom headers to trailers (gRPC-Web sends custom headers as trailers)
		for _, value := range values {
			trailers.Add(key, value)
		}
	}

	// Write trailers
	trailerData := formatTrailerFrame(trailers)
	return writer.writeTrailerFrame(trailerData)
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

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
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
