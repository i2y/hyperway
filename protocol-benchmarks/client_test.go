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

// BenchmarkHyperwayClientStreaming benchmarks Hyperway client streaming with gRPC protocol
func BenchmarkHyperwayClientStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
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
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result (1+2+...+100 = 5050)
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}

// BenchmarkConnectGoClientStreaming benchmarks Connect-go client streaming with gRPC protocol
func BenchmarkConnectGoClientStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
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
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}

// BenchmarkHyperwayBidiStreaming benchmarks Hyperway bidirectional streaming with gRPC protocol
func BenchmarkHyperwayBidiStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
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
			stream := client.EchoStream(ctx)

			// Send and receive 50 messages
			for i := int32(0); i < 50; i++ {
				// Send
				err := stream.Send(&grpcwebv1.EchoRequest{
					Message: "Hello",
					Index:   i,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Receive
				resp, err := stream.Receive()
				if err != nil {
					b.Fatal(err)
				}

				if resp.Index != i {
					b.Fatalf("expected index %d, got %d", i, resp.Index)
				}
			}

			// Close stream
			if err := stream.CloseRequest(); err != nil {
				b.Fatal(err)
			}
			if err := stream.CloseResponse(); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConnectGoBidiStreaming benchmarks Connect-go bidirectional streaming with gRPC protocol
func BenchmarkConnectGoBidiStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
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
			stream := client.EchoStream(ctx)

			// Send and receive 50 messages
			for i := int32(0); i < 50; i++ {
				// Send
				err := stream.Send(&grpcwebv1.EchoRequest{
					Message: "Hello",
					Index:   i,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Receive
				resp, err := stream.Receive()
				if err != nil {
					b.Fatal(err)
				}

				if resp.Index != i {
					b.Fatalf("expected index %d, got %d", i, resp.Index)
				}
			}

			// Close stream
			if err := stream.CloseRequest(); err != nil {
				b.Fatal(err)
			}
			if err := stream.CloseResponse(); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkHyperwayConnectBidiStreaming benchmarks Hyperway bidirectional streaming with Connect protocol
func BenchmarkHyperwayConnectBidiStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		// No WithGRPC() - uses Connect protocol
	)

	b.ResetTimer()
	// Run serially to avoid issues with HTTP/1.1 connection multiplexing
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		stream := client.EchoStream(ctx)

		// Send and receive 50 messages
		for j := int32(0); j < 50; j++ {
			// Send
			err := stream.Send(&grpcwebv1.EchoRequest{
				Message: "Hello",
				Index:   j,
			})
			if err != nil {
				b.Fatal(err)
			}

			// Receive
			resp, err := stream.Receive()
			if err != nil {
				b.Fatal(err)
			}

			if resp.Index != j {
				b.Fatalf("expected index %d, got %d", j, resp.Index)
			}
		}

		// Close stream
		if err := stream.CloseRequest(); err != nil {
			b.Fatal(err)
		}
		if err := stream.CloseResponse(); err != nil && err != io.EOF {
			b.Fatal(err)
		}
	}
}

// BenchmarkConnectGoConnectBidiStreaming benchmarks Connect-go bidirectional streaming with Connect protocol
func BenchmarkConnectGoConnectBidiStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.EchoStream(ctx)

			// Send and receive 50 messages
			for i := int32(0); i < 50; i++ {
				// Send
				err := stream.Send(&grpcwebv1.EchoRequest{
					Message: "Hello",
					Index:   i,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Receive
				resp, err := stream.Receive()
				if err != nil {
					b.Fatal(err)
				}

				if resp.Index != i {
					b.Fatalf("expected index %d, got %d", i, resp.Index)
				}
			}

			// Close stream
			if err := stream.CloseRequest(); err != nil {
				b.Fatal(err)
			}
			if err := stream.CloseResponse(); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkHyperwayConnectBidiStreamingHTTP2 benchmarks Hyperway bidirectional streaming with Connect protocol over HTTP/2
func BenchmarkHyperwayConnectBidiStreamingHTTP2(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.EchoStream(ctx)

			// Send and receive 50 messages
			for i := int32(0); i < 50; i++ {
				// Send
				err := stream.Send(&grpcwebv1.EchoRequest{
					Message: "Hello",
					Index:   i,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Receive
				resp, err := stream.Receive()
				if err != nil {
					b.Fatal(err)
				}

				if resp.Index != i {
					b.Fatalf("expected index %d, got %d", i, resp.Index)
				}
			}

			// Close stream
			if err := stream.CloseRequest(); err != nil {
				b.Fatal(err)
			}
			if err := stream.CloseResponse(); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConnectGoConnectBidiStreamingHTTP2 benchmarks Connect-go bidirectional streaming with Connect protocol over HTTP/2
func BenchmarkConnectGoConnectBidiStreamingHTTP2(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.EchoStream(ctx)

			// Send and receive 50 messages
			for i := int32(0); i < 50; i++ {
				// Send
				err := stream.Send(&grpcwebv1.EchoRequest{
					Message: "Hello",
					Index:   i,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Receive
				resp, err := stream.Receive()
				if err != nil {
					b.Fatal(err)
				}

				if resp.Index != i {
					b.Fatalf("expected index %d, got %d", i, resp.Index)
				}
			}

			// Close stream
			if err := stream.CloseRequest(); err != nil {
				b.Fatal(err)
			}
			if err := stream.CloseResponse(); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkHyperwayConnectClientStreaming benchmarks Hyperway client streaming with Connect protocol
func BenchmarkHyperwayConnectClientStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result (1+2+...+100 = 5050)
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}

// BenchmarkConnectGoConnectClientStreaming benchmarks Connect-go client streaming with Connect protocol
func BenchmarkConnectGoConnectClientStreaming(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}

// BenchmarkHyperwayConnectClientStreamingHTTP2 benchmarks Hyperway client streaming with Connect protocol over HTTP/2
func BenchmarkHyperwayConnectClientStreamingHTTP2(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8080",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result (1+2+...+100 = 5050)
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}

// BenchmarkConnectGoConnectClientStreamingHTTP2 benchmarks Connect-go client streaming with Connect protocol over HTTP/2
func BenchmarkConnectGoConnectClientStreamingHTTP2(b *testing.B) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	client := genconnect.NewGreeterServiceClient(
		httpClient,
		"http://localhost:8084",
		// No WithGRPC() - uses Connect protocol
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := client.SumNumbers(ctx)

			// Send 100 numbers
			for i := int32(1); i <= 100; i++ {
				err := stream.Send(&grpcwebv1.SumRequest{Number: i})
				if err != nil {
					b.Fatal(err)
				}
			}

			// Close send and get response
			resp, err := stream.CloseAndReceive()
			if err != nil {
				b.Fatal(err)
			}

			// Verify result
			if resp.Msg.Total != 5050 {
				b.Fatalf("expected total 5050, got %d", resp.Msg.Total)
			}
			if resp.Msg.Count != 100 {
				b.Fatalf("expected count 100, got %d", resp.Msg.Count)
			}
		}
	})
}
