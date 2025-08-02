// Package main demonstrates multi-protocol support on the same port.
// This example shows HTTP/1.1 and HTTP/2 with Connect RPC and gRPC all working together.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Constants
const (
	maxConcurrentStreams = 100
	httpReadTimeout      = 30 * time.Second
	httpWriteTimeout     = 30 * time.Second
	httpIdleTimeout      = 120 * time.Second
	httpHeaderTimeout    = 5 * time.Second
)

// Simple echo service for testing
type EchoRequest struct {
	Message string `json:"message" validate:"required"`
	// Add protocol info for debugging
	ClientInfo string `json:"client_info,omitempty"`
}

type EchoResponse struct {
	Message      string    `json:"message"`
	ReceivedAt   time.Time `json:"received_at"`
	ProtocolInfo string    `json:"protocol_info"`
}

// Handler that reports which protocol was used
func echoHandler(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	// In a real implementation, we could detect the protocol from context
	// For now, we'll echo back the client info
	protocolInfo := "Protocol detection info"
	if req.ClientInfo != "" {
		protocolInfo = fmt.Sprintf("Client reported: %s", req.ClientInfo)
	}

	return &EchoResponse{
		Message:      req.Message,
		ReceivedAt:   time.Now(),
		ProtocolInfo: protocolInfo,
	}, nil
}

func main() {
	// Create service with all features enabled
	svc := rpc.NewService("EchoService",
		rpc.WithPackage("multiprotocol.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true), // Enable gRPC reflection
		rpc.WithDescription("Multi-protocol echo service for testing"),
	)

	// Register the echo method
	if err := rpc.Register(svc, "Echo", echoHandler); err != nil {
		log.Fatalf("Failed to register Echo: %v", err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Create server with h2c to support both HTTP/1.1 and HTTP/2
	srv := &http.Server{
		Addr: ":9090",
		Handler: h2c.NewHandler(gateway, &http2.Server{
			MaxConcurrentStreams: maxConcurrentStreams,
		}),
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		ReadHeaderTimeout: httpHeaderTimeout,
	}

	log.Println("Multi-protocol server starting on :9090")
	log.Println("This server supports all of the following on the SAME PORT:")
	log.Println("")
	log.Println("1. Connect RPC with JSON (HTTP/1.1):")
	log.Println(`   curl -X POST http://localhost:9090/multiprotocol.v1.EchoService/Echo \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"message":"Hello from curl","client_info":"curl HTTP/1.1"}'`)
	log.Println("")
	log.Println("2. Connect RPC with JSON (HTTP/2):")
	log.Println(`   curl --http2-prior-knowledge -X POST http://localhost:9090/multiprotocol.v1.EchoService/Echo \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"message":"Hello from curl HTTP/2","client_info":"curl HTTP/2"}'`)
	log.Println("")
	log.Println("3. Connect RPC with explicit protocol header:")
	log.Println(`   curl -X POST http://localhost:9090/multiprotocol.v1.EchoService/Echo \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -H "Connect-Protocol-Version: 1" \`)
	log.Println(`     -d '{"message":"Hello from Connect","client_info":"Connect RPC"}'`)
	log.Println("")
	log.Println("4. gRPC with Protobuf (HTTP/2 only):")
	log.Println(`   grpcurl -plaintext -d '{"message":"Hello from grpcurl","client_info":"grpcurl"}' \`)
	log.Println(`     localhost:9090 multiprotocol.v1.EchoService/Echo`)
	log.Println("")
	log.Println("5. gRPC reflection:")
	log.Println(`   grpcurl -plaintext localhost:9090 list`)
	log.Println(`   grpcurl -plaintext localhost:9090 describe multiprotocol.v1.EchoService`)
	log.Println("")

	log.Fatal(srv.ListenAndServe())
}
