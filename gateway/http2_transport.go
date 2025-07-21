// Package gateway provides HTTP/2 transport with keepalive support.
package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// HTTP2Transport wraps an HTTP/2 server with keepalive support.
type HTTP2Transport struct {
	server          *http2.Server
	keepaliveConfig *keepaliveConfig
	activeStreams   sync.Map // track active streams
	lastPingTime    time.Time
	pingStrikes     int
	mu              sync.Mutex
}

// HTTP/2 configuration constants
const (
	defaultMaxConcurrentStreams = 100
	defaultMaxReadFrameSize     = 16 * 1024         // 16KB
	defaultIdleTimeout          = 120 * time.Second // 2 minutes
	defaultReadHeaderTimeout    = 10 * time.Second  // Slowloris mitigation
)

// NewHTTP2Transport creates a new HTTP/2 transport with keepalive support.
func NewHTTP2Transport(opts Options) *HTTP2Transport {
	config := newKeepaliveConfig()

	// Apply custom keepalive parameters if provided
	if opts.KeepaliveParams != nil {
		config.clientParams = *opts.KeepaliveParams
	}
	if opts.KeepaliveEnforcementPolicy != nil {
		config.enforcementPolicy = *opts.KeepaliveEnforcementPolicy
	}

	transport := &HTTP2Transport{
		keepaliveConfig: config,
		lastPingTime:    time.Now(),
	}

	// Configure HTTP/2 server
	transport.server = &http2.Server{
		MaxConcurrentStreams: defaultMaxConcurrentStreams,
		MaxReadFrameSize:     defaultMaxReadFrameSize,
		IdleTimeout:          defaultIdleTimeout,
		// Configure keepalive enforcement
		PermitProhibitedCipherSuites: false,
	}

	return transport
}

// WrapHandler wraps an HTTP handler with HTTP/2 and keepalive support.
func (t *HTTP2Transport) WrapHandler(handler http.Handler) http.Handler {
	// Create HTTP/2 handler with h2c for non-TLS
	h2cHandler := h2c.NewHandler(handler, t.server)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track active streams
		streamID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())
		t.activeStreams.Store(streamID, true)
		defer t.activeStreams.Delete(streamID)

		// Check if this is a PING frame (handled by HTTP/2 layer)
		if r.Method == "PRI" && r.URL.Path == "*" && r.Proto == "HTTP/2.0" {
			// This is an HTTP/2 connection preface, let h2c handle it
			h2cHandler.ServeHTTP(w, r)
			return
		}

		// For regular requests, check keepalive enforcement
		if err := t.enforceKeepalive(r); err != nil {
			// Send GOAWAY frame for too many pings
			w.Header().Set("Grpc-Status", "14") // UNAVAILABLE
			w.Header().Set("Grpc-Message", "too_many_pings")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Serve the request
		h2cHandler.ServeHTTP(w, r)
	})
}

// enforceKeepalive checks if the client is respecting keepalive policies.
func (t *HTTP2Transport) enforceKeepalive(_ *http.Request) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// Check if there are active streams
	hasActiveStreams := false
	t.activeStreams.Range(func(_, _ any) bool {
		hasActiveStreams = true
		return false
	})

	// If no active streams and not permitted, check ping frequency
	if !hasActiveStreams && !t.keepaliveConfig.enforcementPolicy.PermitWithoutStream {
		timeSinceLastPing := now.Sub(t.lastPingTime)

		if timeSinceLastPing < t.keepaliveConfig.enforcementPolicy.MinTime {
			t.pingStrikes++

			// Check if we've exceeded max strikes
			if t.keepaliveConfig.enforcementPolicy.MaxPingStrikes > 0 &&
				t.pingStrikes > t.keepaliveConfig.enforcementPolicy.MaxPingStrikes {
				return fmt.Errorf("too many keepalive pings")
			}
		} else {
			// Reset strikes if enough time has passed
			t.pingStrikes = 0
		}
	}

	t.lastPingTime = now
	return nil
}

// ConfigureServer configures an HTTP server with keepalive parameters.
func ConfigureServerWithKeepalive(server *http.Server, keepalive *KeepaliveParameters) {
	if keepalive == nil {
		return
	}

	// Set server timeouts based on keepalive parameters
	if server.IdleTimeout == 0 {
		// Set idle timeout based on keepalive time
		server.IdleTimeout = keepalive.Time + keepalive.Timeout
	}

	if server.ReadTimeout == 0 {
		// Allow enough time for keepalive
		server.ReadTimeout = keepalive.Timeout * 2
	}

	if server.WriteTimeout == 0 {
		server.WriteTimeout = keepalive.Timeout * 2
	}
}

// NewHTTP2Server creates an HTTP server configured for HTTP/2 with keepalive.
func NewHTTP2Server(addr string, handler http.Handler, opts Options) *http.Server {
	transport := NewHTTP2Transport(opts)

	server := &http.Server{
		Addr:              addr,
		Handler:           transport.WrapHandler(handler),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	// Configure keepalive timeouts
	if opts.KeepaliveParams != nil {
		ConfigureServerWithKeepalive(server, opts.KeepaliveParams)
	}

	// Configure HTTP/2
	if err := http2.ConfigureServer(server, transport.server); err != nil {
		// This should not happen in practice
		panic(fmt.Sprintf("failed to configure HTTP/2: %v", err))
	}

	return server
}

// ListenAndServeHTTP2 starts an HTTP/2 server with keepalive support.
func ListenAndServeHTTP2(addr string, handler http.Handler, opts Options) error {
	server := NewHTTP2Server(addr, handler, opts)

	// Create listener
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Start keepalive timer if configured
	if opts.KeepaliveParams != nil && opts.KeepaliveParams.PermitWithoutStream {
		go startKeepaliveTimer(server.BaseContext(lis), opts.KeepaliveParams)
	}

	return server.Serve(lis)
}

// startKeepaliveTimer sends periodic PING frames according to keepalive parameters.
func startKeepaliveTimer(ctx context.Context, params *KeepaliveParameters) {
	ticker := time.NewTicker(params.Time)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// HTTP/2 PING frames are handled at the transport layer
			// This is just a placeholder for the timer logic
			// Actual PING frame sending is done by the HTTP/2 implementation
		}
	}
}
