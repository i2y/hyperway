// Package gateway provides keepalive support for gRPC according to the specification.
package gateway

import (
	"time"
)

// KeepaliveParameters configures gRPC keepalive according to the specification.
// These settings control HTTP/2 PING frames for connection health checking.
type KeepaliveParameters struct {
	// Time after which a keepalive ping is sent on the transport.
	// Default: 2 hours (7200000ms)
	Time time.Duration

	// Timeout for keepalive ping acknowledgement.
	// If no acknowledgement is received, the connection is closed.
	// Default: 20 seconds (20000ms)
	Timeout time.Duration

	// If true, keepalive pings are sent even without active calls.
	// Default: false
	PermitWithoutStream bool

	// Maximum number of pings that can be sent when there is no data/header frame to be sent.
	// Default: 2
	MaxPingsWithoutData int
}

// KeepaliveEnforcementPolicy configures server-side keepalive enforcement.
type KeepaliveEnforcementPolicy struct {
	// Minimum time between receiving successive pings without data/header frames.
	// If pings are received more frequently, they are considered bad pings.
	// Default: 5 minutes
	MinTime time.Duration

	// If true, server allows keepalive pings even when there are no active streams.
	// Default: false
	PermitWithoutStream bool

	// Maximum number of bad pings before closing the connection.
	// 0 means the server will tolerate any number of bad pings.
	// Default: 2
	MaxPingStrikes int
}

// Keepalive timeout constants
const (
	defaultKeepaliveTimeoutShort = 20 * time.Second
)

// DefaultKeepaliveParams returns default client-side keepalive parameters.
func DefaultKeepaliveParams() KeepaliveParameters {
	return KeepaliveParameters{
		Time:                2 * time.Hour,
		Timeout:             defaultKeepaliveTimeoutShort,
		PermitWithoutStream: false,
		MaxPingsWithoutData: 2,
	}
}

// DefaultKeepaliveEnforcementPolicy returns default server-side enforcement policy.
func DefaultKeepaliveEnforcementPolicy() KeepaliveEnforcementPolicy {
	return KeepaliveEnforcementPolicy{
		MinTime:             5 * time.Minute,
		PermitWithoutStream: false,
		MaxPingStrikes:      2,
	}
}

// AggressiveKeepaliveParams returns more aggressive keepalive parameters for
// environments with proxies that kill idle connections.
func AggressiveKeepaliveParams() KeepaliveParameters {
	return KeepaliveParameters{
		Time:                30 * time.Second, // Send ping every 30 seconds
		Timeout:             10 * time.Second, // Timeout after 10 seconds
		PermitWithoutStream: true,             // Allow pings without active calls
		MaxPingsWithoutData: 2,
	}
}

// keepaliveConfig holds the complete keepalive configuration.
type keepaliveConfig struct {
	clientParams      KeepaliveParameters
	enforcementPolicy KeepaliveEnforcementPolicy
}

// newKeepaliveConfig creates a new keepalive configuration with defaults.
func newKeepaliveConfig() *keepaliveConfig {
	return &keepaliveConfig{
		clientParams:      DefaultKeepaliveParams(),
		enforcementPolicy: DefaultKeepaliveEnforcementPolicy(),
	}
}
