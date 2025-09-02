package rpc

// ProtocolPreset represents a predefined set of protocol configurations.
type ProtocolPreset string

const (
	// PresetREST enables Connect and JSON-RPC protocols for REST-like APIs.
	PresetREST ProtocolPreset = "rest"

	// PresetGRPC enables gRPC and gRPC-Web protocols.
	PresetGRPC ProtocolPreset = "grpc"

	// PresetAll enables all available protocols.
	PresetAll ProtocolPreset = "all"

	// PresetMinimal enables only Connect protocol.
	PresetMinimal ProtocolPreset = "minimal"
)

// WithConnect enables Connect protocol with specified settings.
func WithConnect(allowJSON, allowProto bool) ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["connect"] = ProtocolConfig{
			Name:    "connect",
			Enabled: true,
			Settings: ConnectSettings{
				AllowJSON:  allowJSON,
				AllowProto: allowProto,
			},
		}
	}
}

// WithGRPC enables gRPC protocol with optional reflection.
func WithGRPC(enableReflection bool) ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["grpc"] = ProtocolConfig{
			Name:    "grpc",
			Enabled: true,
			Settings: GRPCSettings{
				EnableReflection: enableReflection,
			},
		}
		// Also set the reflection option for backward compatibility
		o.EnableReflection = enableReflection
	}
}

// WithGRPCWeb enables gRPC-Web protocol.
func WithGRPCWeb() ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["grpcweb"] = ProtocolConfig{
			Name:     "grpcweb",
			Enabled:  true,
			Settings: GRPCWebSettings{},
		}
	}
}

// WithJSONRPC enables JSON-RPC protocol with specified settings.
func WithJSONRPC(path string, batchLimit int) ServiceOption {
	if path == "" {
		path = DefaultJSONRPCPath
	}
	if batchLimit <= 0 {
		batchLimit = DefaultJSONRPCBatchLimit
	}

	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["jsonrpc"] = ProtocolConfig{
			Name:    "jsonrpc",
			Enabled: true,
			Settings: JSONRPCSettings{
				Path:       path,
				BatchLimit: batchLimit,
			},
		}
	}
}

// DisableConnect disables Connect protocol.
func DisableConnect() ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["connect"] = ProtocolConfig{
			Name:    "connect",
			Enabled: false,
		}
	}
}

// DisableGRPC disables gRPC protocol.
func DisableGRPC() ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["grpc"] = ProtocolConfig{
			Name:    "grpc",
			Enabled: false,
		}
		// Also disable reflection
		o.EnableReflection = false
	}
}

// DisableGRPCWeb disables gRPC-Web protocol.
func DisableGRPCWeb() ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["grpcweb"] = ProtocolConfig{
			Name:    "grpcweb",
			Enabled: false,
		}
	}
}

// DisableJSONRPC disables JSON-RPC protocol.
func DisableJSONRPC() ServiceOption {
	return func(o *ServiceOptions) {
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		o.EnabledProtocols["jsonrpc"] = ProtocolConfig{
			Name:    "jsonrpc",
			Enabled: false,
		}
	}
}

// WithPreset applies a predefined set of protocol configurations.
func WithPreset(preset ProtocolPreset) ServiceOption {
	return func(o *ServiceOptions) {
		switch preset {
		case PresetREST:
			// Enable Connect and JSON-RPC for REST-like APIs
			WithConnect(true, true)(o)
			WithJSONRPC(DefaultJSONRPCPath, DefaultJSONRPCBatchLimit)(o)
			DisableGRPC()(o)
			DisableGRPCWeb()(o)

		case PresetGRPC:
			// Enable gRPC and gRPC-Web
			WithGRPC(true)(o)
			WithGRPCWeb()(o)
			DisableConnect()(o)
			DisableJSONRPC()(o)

		case PresetAll:
			// Enable all protocols
			WithConnect(true, true)(o)
			WithGRPC(true)(o)
			WithGRPCWeb()(o)
			WithJSONRPC(DefaultJSONRPCPath, DefaultJSONRPCBatchLimit)(o)

		case PresetMinimal:
			// Enable only Connect
			WithConnect(true, true)(o)
			DisableGRPC()(o)
			DisableGRPCWeb()(o)
			DisableJSONRPC()(o)
		}
	}
}

// ProtocolConfigBuilder provides a fluent API for configuring protocols.
type ProtocolConfigBuilder struct {
	options ServiceOptions
}

// ConfigureProtocols creates a new protocol configuration builder.
func ConfigureProtocols() *ProtocolConfigBuilder {
	return &ProtocolConfigBuilder{
		options: ServiceOptions{
			EnabledProtocols: make(map[string]ProtocolConfig),
		},
	}
}

// Connect configures Connect protocol.
func (b *ProtocolConfigBuilder) Connect(allowJSON, allowProto bool) *ProtocolConfigBuilder {
	b.options.EnabledProtocols["connect"] = ProtocolConfig{
		Name:    "connect",
		Enabled: true,
		Settings: ConnectSettings{
			AllowJSON:  allowJSON,
			AllowProto: allowProto,
		},
	}
	return b
}

// GRPC configures gRPC protocol.
func (b *ProtocolConfigBuilder) GRPC(enableReflection bool) *ProtocolConfigBuilder {
	b.options.EnabledProtocols["grpc"] = ProtocolConfig{
		Name:    "grpc",
		Enabled: true,
		Settings: GRPCSettings{
			EnableReflection: enableReflection,
		},
	}
	b.options.EnableReflection = enableReflection
	return b
}

// GRPCWeb configures gRPC-Web protocol.
func (b *ProtocolConfigBuilder) GRPCWeb() *ProtocolConfigBuilder {
	b.options.EnabledProtocols["grpcweb"] = ProtocolConfig{
		Name:     "grpcweb",
		Enabled:  true,
		Settings: GRPCWebSettings{},
	}
	return b
}

// JSONRPC configures JSON-RPC protocol.
func (b *ProtocolConfigBuilder) JSONRPC(path string, batchLimit int) *ProtocolConfigBuilder {
	if path == "" {
		path = DefaultJSONRPCPath
	}
	if batchLimit <= 0 {
		batchLimit = DefaultJSONRPCBatchLimit
	}

	b.options.EnabledProtocols["jsonrpc"] = ProtocolConfig{
		Name:    "jsonrpc",
		Enabled: true,
		Settings: JSONRPCSettings{
			Path:       path,
			BatchLimit: batchLimit,
		},
	}
	return b
}

// Build creates a ServiceOption from the builder.
func (b *ProtocolConfigBuilder) Build() ServiceOption {
	return func(o *ServiceOptions) {
		// Merge protocols
		if o.EnabledProtocols == nil {
			o.EnabledProtocols = make(map[string]ProtocolConfig)
		}
		for name, config := range b.options.EnabledProtocols {
			o.EnabledProtocols[name] = config
		}
		// Apply other options
		if b.options.EnableReflection {
			o.EnableReflection = true
		}
	}
}

// EnableAllProtocols is a convenience function that enables all protocols with default settings.
func EnableAllProtocols() ServiceOption {
	return WithPreset(PresetAll)
}

// EnableRESTProtocols enables protocols suitable for REST-like APIs.
func EnableRESTProtocols() ServiceOption {
	return WithPreset(PresetREST)
}

// EnableGRPCProtocols enables gRPC-related protocols.
func EnableGRPCProtocols() ServiceOption {
	return WithPreset(PresetGRPC)
}
