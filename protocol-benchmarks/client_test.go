package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"encoding/json"
	"golang.org/x/net/http2"

	grpcwebv1 "grpc-real-comparison/gen"
	"grpc-real-comparison/gen/genconnect"
)

// JSONCodec implements connect.Codec for JSON
type JSONCodec struct{}

func (c *JSONCodec) Name() string { return "json" }

func (c *JSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (c *JSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func BenchmarkConnectGoGRPC(b *testing.B) {
	// Create Connect client with gRPC protocol
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		connect.WithGRPC(),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHyperwayGRPC(b *testing.B) {
	// Create Connect client with gRPC protocol for Hyperway
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		connect.WithGRPC(),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConnectGoConnect(b *testing.B) {
	// Create Connect client with Connect protocol
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHyperwayConnect(b *testing.B) {
	// Create Connect client with Connect protocol for Hyperway
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConnectGoConnectJSON(b *testing.B) {
	// Create Connect client with Connect protocol and JSON codec
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		connect.WithCodec(&JSONCodec{}),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHyperwayConnectJSON(b *testing.B) {
	// Create Connect client with Connect protocol and JSON codec for Hyperway
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		connect.WithCodec(&JSONCodec{}),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConnectGoStreaming(b *testing.B) {
	// Create Connect client with Connect protocol
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.StreamRequest{Count: 100})
			stream, err := client.StreamNumbers(ctx, req)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for stream.Receive() {
				count++
			}

			if err := stream.Err(); err != nil && err != io.EOF {
				b.Fatal(err)
			}

			if count != 100 {
				b.Fatalf("expected 100 messages, got %d", count)
			}
		}
	})
}

func BenchmarkHyperwayStreaming(b *testing.B) {
	// Create Connect client with Connect protocol for Hyperway
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.StreamRequest{Count: 100})
			stream, err := client.StreamNumbers(ctx, req)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for stream.Receive() {
				count++
			}

			if err := stream.Err(); err != nil && err != io.EOF {
				b.Fatal(err)
			}

			if count != 100 {
				b.Fatalf("expected 100 messages, got %d", count)
			}
		}
	})
}

func BenchmarkConnectGoConnectProtoHTTP2(b *testing.B) {
	// Create Connect client with Connect protocol and HTTP/2
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHyperwayConnectProtoHTTP2(b *testing.B) {
	// Create Connect client with Connect protocol and HTTP/2 for Hyperway
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConnectGoConnectJSONHTTP2(b *testing.B) {
	// Create Connect client with Connect protocol, JSON codec and HTTP/2
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		connect.WithCodec(&JSONCodec{}),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHyperwayConnectJSONHTTP2(b *testing.B) {
	// Create Connect client with Connect protocol, JSON codec and HTTP/2 for Hyperway
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		connect.WithCodec(&JSONCodec{}),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.GreetRequest{Name: "World"})
			_, err := client.Greet(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConnectGoGRPCStreaming(b *testing.B) {
	// Create Connect client with gRPC protocol for streaming
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		connect.WithGRPC(),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.StreamRequest{Count: 100})
			stream, err := client.StreamNumbers(ctx, req)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for stream.Receive() {
				count++
			}

			if err := stream.Err(); err != nil && err != io.EOF {
				b.Fatal(err)
			}

			if count != 100 {
				b.Fatalf("expected 100 messages, got %d", count)
			}
		}
	})
}

func BenchmarkHyperwayGRPCStreaming(b *testing.B) {
	// Create Connect client with gRPC protocol for Hyperway streaming
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Force HTTP/2 without TLS
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		connect.WithGRPC(),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := connect.NewRequest(&grpcwebv1.StreamRequest{Count: 100})
			stream, err := client.StreamNumbers(ctx, req)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for stream.Receive() {
				count++
			}

			if err := stream.Err(); err != nil && err != io.EOF {
				b.Fatal(err)
			}

			if count != 100 {
				b.Fatalf("expected 100 messages, got %d", count)
			}
		}
	})
}
