package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"

	grpcwebv1 "grpc-real-comparison/gen"
	"grpc-real-comparison/gen/genconnect"
)

type GreeterService struct{}

func (s *GreeterService) Greet(
	ctx context.Context,
	req *connect.Request[grpcwebv1.GreetRequest],
) (*connect.Response[grpcwebv1.GreetResponse], error) {
	return connect.NewResponse(&grpcwebv1.GreetResponse{
		Message:   fmt.Sprintf("Hello, %s! Welcome to Connect-Go!", req.Msg.Name),
		Timestamp: timestamppb.Now(),
	}), nil
}

func (s *GreeterService) Calculate(
	ctx context.Context,
	req *connect.Request[grpcwebv1.CalculateRequest],
) (*connect.Response[grpcwebv1.CalculateResponse], error) {
	var result float64
	switch req.Msg.Operator {
	case "+":
		result = req.Msg.A + req.Msg.B
	case "-":
		result = req.Msg.A - req.Msg.B
	case "*":
		result = req.Msg.A * req.Msg.B
	case "/":
		if req.Msg.B != 0 {
			result = req.Msg.A / req.Msg.B
		}
	}

	return connect.NewResponse(&grpcwebv1.CalculateResponse{
		Result:  result,
		Formula: fmt.Sprintf("%f %s %f = %f", req.Msg.A, req.Msg.Operator, req.Msg.B, result),
	}), nil
}

func (s *GreeterService) StreamNumbers(
	ctx context.Context,
	req *connect.Request[grpcwebv1.StreamRequest],
	stream *connect.ServerStream[grpcwebv1.NumberResponse],
) error {
	for i := int32(1); i <= req.Msg.Count; i++ {
		resp := &grpcwebv1.NumberResponse{
			Number:    i,
			Timestamp: timestamppb.Now(),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	greeter := &GreeterService{}

	// Create gRPC handler using generated code
	path, handler := genconnect.NewGreeterServiceHandler(greeter)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	// Configure HTTP/2 with keep-alive
	h2s := &http2.Server{
		IdleTimeout: 120 * time.Second,
	}

	server := &http.Server{
		Addr:         ":8084",
		Handler:      h2c.NewHandler(mux, h2s),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("Connect-Go gRPC server with keep-alive listening on :8084")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
