package rpc

// Protocol represents a supported RPC protocol
type Protocol interface {
	Name() string
	Config() ProtocolConfig
}

// ProtocolConfig holds configuration for a specific protocol
type ProtocolConfig struct {
	Name     string
	Enabled  bool
	Settings interface{}
}

// ConnectSettings holds Connect-specific settings
type ConnectSettings struct {
	AllowJSON  bool
	AllowProto bool
}

// GRPCSettings holds gRPC-specific settings
type GRPCSettings struct {
	EnableReflection bool
}

// GRPCWebSettings holds gRPC-Web-specific settings
type GRPCWebSettings struct{}

// JSONRPCSettings holds JSON-RPC-specific settings
type JSONRPCSettings struct {
	Path       string
	BatchLimit int
}

// connectProtocol implements Protocol for Connect
type connectProtocol struct {
	allowJSON  bool
	allowProto bool
}

func (c *connectProtocol) Name() string {
	return "connect"
}

func (c *connectProtocol) Config() ProtocolConfig {
	return ProtocolConfig{
		Name:    "connect",
		Enabled: true,
		Settings: ConnectSettings{
			AllowJSON:  c.allowJSON,
			AllowProto: c.allowProto,
		},
	}
}

// grpcProtocol implements Protocol for gRPC
type grpcProtocol struct {
	enableReflection bool
}

func (g *grpcProtocol) Name() string {
	return "grpc"
}

func (g *grpcProtocol) Config() ProtocolConfig {
	return ProtocolConfig{
		Name:    "grpc",
		Enabled: true,
		Settings: GRPCSettings{
			EnableReflection: g.enableReflection,
		},
	}
}

// grpcWebProtocol implements Protocol for gRPC-Web
type grpcWebProtocol struct{}

func (g *grpcWebProtocol) Name() string {
	return "grpcweb"
}

func (g *grpcWebProtocol) Config() ProtocolConfig {
	return ProtocolConfig{
		Name:     "grpcweb",
		Enabled:  true,
		Settings: GRPCWebSettings{},
	}
}

// jsonrpcProtocol implements Protocol for JSON-RPC
type jsonrpcProtocol struct {
	path       string
	batchLimit int
}

func (j *jsonrpcProtocol) Name() string {
	return "jsonrpc"
}

func (j *jsonrpcProtocol) Config() ProtocolConfig {
	return ProtocolConfig{
		Name:    "jsonrpc",
		Enabled: true,
		Settings: JSONRPCSettings{
			Path:       j.path,
			BatchLimit: j.batchLimit,
		},
	}
}

// ConnectOption configures Connect protocol
type ConnectOption func(*connectProtocol)

// WithConnectJSON enables/disables JSON support for Connect
func WithConnectJSON(allow bool) ConnectOption {
	return func(c *connectProtocol) {
		c.allowJSON = allow
	}
}

// WithConnectProto enables/disables Protobuf support for Connect
func WithConnectProto(allow bool) ConnectOption {
	return func(c *connectProtocol) {
		c.allowProto = allow
	}
}

// Connect creates a Connect protocol configuration
func Connect(opts ...ConnectOption) Protocol {
	c := &connectProtocol{
		allowJSON:  true,
		allowProto: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GRPCOption configures gRPC protocol
type GRPCOption func(*grpcProtocol)

// WithGRPCReflection enables/disables gRPC reflection
func WithGRPCReflection(enable bool) GRPCOption {
	return func(g *grpcProtocol) {
		g.enableReflection = enable
	}
}

// GRPC creates a gRPC protocol configuration
func GRPC(opts ...GRPCOption) Protocol {
	g := &grpcProtocol{
		enableReflection: true, // Default to true for backward compatibility
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// GRPCWeb creates a gRPC-Web protocol configuration
func GRPCWeb() Protocol {
	return &grpcWebProtocol{}
}

// JSONRPCOption configures JSON-RPC protocol
type JSONRPCOption func(*jsonrpcProtocol)

// WithBatchLimit sets the batch request limit for JSON-RPC
func WithBatchLimit(limit int) JSONRPCOption {
	return func(j *jsonrpcProtocol) {
		j.batchLimit = limit
	}
}

const (
	// DefaultJSONRPCPath is the default path for JSON-RPC endpoints
	DefaultJSONRPCPath = "/jsonrpc"
	// DefaultJSONRPCBatchLimit is the default batch request limit
	DefaultJSONRPCBatchLimit = 50
)

// JSONRPC creates a JSON-RPC protocol configuration
func JSONRPC(path string, opts ...JSONRPCOption) Protocol {
	if path == "" {
		path = DefaultJSONRPCPath
	}
	j := &jsonrpcProtocol{
		path:       path,
		batchLimit: DefaultJSONRPCBatchLimit,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}
