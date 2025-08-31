package rpc

import (
	"context"
	"fmt"
)

// ContextHelper provides a fluent API for working with handler context.
type ContextHelper struct {
	ctx         context.Context
	handlerCtx  *handlerContext
	validations []func() error
}

// Context creates a new ContextHelper from a context.
func Context(ctx context.Context) *ContextHelper {
	hctx := GetHandlerContext(ctx)
	return &ContextHelper{
		ctx:        ctx,
		handlerCtx: hctx,
	}
}

// SetHeader sets a response header.
func (c *ContextHelper) SetHeader(key, value string) *ContextHelper {
	if c.handlerCtx != nil {
		c.handlerCtx.SetResponseHeader(key, value)
	}
	return c
}

// SetHeaders sets multiple response headers.
func (c *ContextHelper) SetHeaders(headers map[string]string) *ContextHelper {
	if c.handlerCtx != nil {
		for key, value := range headers {
			c.handlerCtx.SetResponseHeader(key, value)
		}
	}
	return c
}

// SetTrailer sets a response trailer.
func (c *ContextHelper) SetTrailer(key, value string) *ContextHelper {
	if c.handlerCtx != nil {
		c.handlerCtx.SetResponseTrailer(key, value)
	}
	return c
}

// SetTrailers sets multiple response trailers.
func (c *ContextHelper) SetTrailers(trailers map[string]string) *ContextHelper {
	if c.handlerCtx != nil {
		for key, value := range trailers {
			c.handlerCtx.SetResponseTrailer(key, value)
		}
	}
	return c
}

// Header gets a request header value.
// Returns the first value if multiple values exist.
func (c *ContextHelper) Header(key string) string {
	if c.handlerCtx != nil {
		values := c.handlerCtx.GetRequestHeader(key)
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// HeaderValues gets all values for a request header.
func (c *ContextHelper) HeaderValues(key string) []string {
	if c.handlerCtx != nil {
		return c.handlerCtx.GetRequestHeader(key)
	}
	return nil
}

// Headers gets all request headers.
func (c *ContextHelper) Headers() map[string][]string {
	if c.handlerCtx != nil {
		return c.handlerCtx.GetRequestHeaders()
	}
	return nil
}

// HasHeader checks if a header exists.
func (c *ContextHelper) HasHeader(key string) bool {
	if c.handlerCtx != nil {
		values := c.handlerCtx.GetRequestHeader(key)
		return len(values) > 0
	}
	return false
}

// RequireHeader adds a validation that the specified header must exist.
func (c *ContextHelper) RequireHeader(key string) *ContextHelper {
	c.validations = append(c.validations, func() error {
		if !c.HasHeader(key) {
			return fmt.Errorf("required header %q is missing", key)
		}
		return nil
	})
	return c
}

// RequireHeaders adds validations that all specified headers must exist.
func (c *ContextHelper) RequireHeaders(keys ...string) *ContextHelper {
	for _, key := range keys {
		c.RequireHeader(key)
	}
	return c
}

// SetMetadata sets a metadata value in the context.
// Metadata is stored in the response headers with a "x-metadata-" prefix.
func (c *ContextHelper) SetMetadata(key, value string) *ContextHelper {
	return c.SetHeader("x-metadata-"+key, value)
}

// Metadata gets a metadata value from the request.
// Looks for headers with "x-metadata-" prefix.
func (c *ContextHelper) Metadata(key string) string {
	return c.Header("x-metadata-" + key)
}

// RequireMetadata adds a validation that the specified metadata must exist.
func (c *ContextHelper) RequireMetadata(key string) *ContextHelper {
	c.validations = append(c.validations, func() error {
		if c.Metadata(key) == "" {
			return fmt.Errorf("required metadata %q is missing", key)
		}
		return nil
	})
	return c
}

// Validate runs all accumulated validations.
// Returns the first error encountered, or nil if all validations pass.
func (c *ContextHelper) Validate() error {
	for _, validation := range c.validations {
		if err := validation(); err != nil {
			return err
		}
	}
	return nil
}

// MustValidate runs all validations and panics if any fail.
func (c *ContextHelper) MustValidate() {
	if err := c.Validate(); err != nil {
		panic(err)
	}
}

// IsGRPC checks if the current request is using gRPC protocol.
func (c *ContextHelper) IsGRPC() bool {
	contentType := c.Header("Content-Type")
	return contentType == "application/grpc" ||
		contentType == "application/grpc+proto" ||
		contentType == "application/grpc+json"
}

// IsConnect checks if the current request is using Connect protocol.
func (c *ContextHelper) IsConnect() bool {
	return c.Header("Connect-Protocol-Version") == "1"
}

// IsGRPCWeb checks if the current request is using gRPC-Web protocol.
func (c *ContextHelper) IsGRPCWeb() bool {
	return c.HasHeader("X-Grpc-Web") || c.HasHeader("grpc-web")
}

// IsJSONRPC checks if the current request is using JSON-RPC protocol.
func (c *ContextHelper) IsJSONRPC() bool {
	contentType := c.Header("Content-Type")
	return contentType == "application/json-rpc" ||
		contentType == "application/json-rpc+json"
}

// ClientIP attempts to get the client's IP address from common headers.
func (c *ContextHelper) ClientIP() string {
	// Check common headers in order of preference
	if ip := c.Header("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.Header("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if comma := len(ip); comma > 0 {
			for i, ch := range ip {
				if ch == ',' {
					return ip[:i]
				}
			}
		}
		return ip
	}
	return c.Header("RemoteAddr")
}

// UserAgent gets the User-Agent header.
func (c *ContextHelper) UserAgent() string {
	return c.Header("User-Agent")
}

// Authorization gets the Authorization header value.
func (c *ContextHelper) Authorization() string {
	return c.Header("Authorization")
}

// BearerToken extracts the bearer token from the Authorization header.
// Returns empty string if the header is not in the format "Bearer <token>".
func (c *ContextHelper) BearerToken() string {
	auth := c.Authorization()
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// Context returns the underlying context.Context.
func (c *ContextHelper) Context() context.Context {
	return c.ctx
}

// HandlerContext returns the underlying handlerContext.
// This is useful for advanced use cases that need direct access.
func (c *ContextHelper) HandlerContext() *handlerContext {
	return c.handlerCtx
}

// WithValue returns a new ContextHelper with the given key-value pair added to the context.
func (c *ContextHelper) WithValue(key, value any) *ContextHelper {
	return &ContextHelper{
		ctx:         context.WithValue(c.ctx, key, value),
		handlerCtx:  c.handlerCtx,
		validations: c.validations,
	}
}
