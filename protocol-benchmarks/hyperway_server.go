//go:build hyperway
// +build hyperway

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service models matching greeter.proto
type GreetRequest struct {
	Name string `json:"name"`
}

type GreetResponse struct {
	Message   string                 `json:"message"`
	Timestamp *timestamppb.Timestamp `json:"timestamp"`
}

type CalculateRequest struct {
	A        float64 `json:"a"`
	B        float64 `json:"b"`
	Operator string  `json:"operator"`
}

type CalculateResponse struct {
	Result  float64 `json:"result"`
	Formula string  `json:"formula"`
}

type StreamRequest struct {
	Count int32 `json:"count"`
}

type NumberResponse struct {
	Number    int32                  `json:"number"`
	Timestamp *timestamppb.Timestamp `json:"timestamp"`
}

type SumRequest struct {
	Number int32 `json:"number"`
}

type SumResponse struct {
	Total int32 `json:"total"`
	Count int32 `json:"count"`
}

type EchoRequest struct {
	Message string `json:"message"`
	Index   int32  `json:"index"`
}

type EchoResponse struct {
	Message   string                 `json:"message"`
	Index     int32                  `json:"index"`
	Timestamp *timestamppb.Timestamp `json:"timestamp"`
}

// Service implementation
type greeterService struct{}

func (s *greeterService) Greet(ctx context.Context, req *GreetRequest) (*GreetResponse, error) {
	return &GreetResponse{
		Message:   fmt.Sprintf("Hello, %s!", req.Name),
		Timestamp: timestamppb.Now(),
	}, nil
}

func (s *greeterService) Calculate(ctx context.Context, req *CalculateRequest) (*CalculateResponse, error) {
	var result float64
	var formula string

	switch req.Operator {
	case "+":
		result = req.A + req.B
		formula = fmt.Sprintf("%.2f + %.2f = %.2f", req.A, req.B, result)
	case "-":
		result = req.A - req.B
		formula = fmt.Sprintf("%.2f - %.2f = %.2f", req.A, req.B, result)
	case "*":
		result = req.A * req.B
		formula = fmt.Sprintf("%.2f * %.2f = %.2f", req.A, req.B, result)
	case "/":
		if req.B == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		result = req.A / req.B
		formula = fmt.Sprintf("%.2f / %.2f = %.2f", req.A, req.B, result)
	default:
		return nil, fmt.Errorf("unsupported operator: %s", req.Operator)
	}

	return &CalculateResponse{
		Result:  result,
		Formula: formula,
	}, nil
}

func (s *greeterService) StreamNumbers(ctx context.Context, req *StreamRequest, stream rpc.ServerStream[NumberResponse]) error {
	for i := int32(1); i <= req.Count; i++ {
		resp := &NumberResponse{
			Number:    i,
			Timestamp: timestamppb.Now(),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func (s *greeterService) SumNumbers(ctx context.Context, stream rpc.ClientStream[SumRequest]) (*SumResponse, error) {
	var total int32
	var count int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// End of stream
			break
		}
		if err != nil {
			return nil, err
		}
		total += req.Number
		count++
	}

	return &SumResponse{
		Total: total,
		Count: count,
	}, nil
}

func (s *greeterService) EchoStream(ctx context.Context, stream rpc.BidiStream[EchoRequest, EchoResponse]) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Client closed the stream
			return nil
		}
		if err != nil {
			return err
		}

		resp := &EchoResponse{
			Message:   "Echo: " + req.Message,
			Index:     req.Index,
			Timestamp: timestamppb.Now(),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func main() {
	// Create service
	svc := rpc.NewService("GreeterService",
		rpc.WithPackage("grpcweb.example.v1"),
		rpc.WithReflection(true))

	// Create service instance
	greeter := &greeterService{}

	// Register methods
	rpc.MustRegisterAs(svc, "Greet", greeter.Greet)
	rpc.MustRegisterAs(svc, "Calculate", greeter.Calculate)

	// Register streaming methods
	rpc.MustRegisterServerStreamAs(svc, "StreamNumbers", greeter.StreamNumbers)
	rpc.MustRegisterClientStreamAs(svc, "SumNumbers", greeter.SumNumbers)
	rpc.MustRegisterBidiStreamAs(svc, "EchoStream", greeter.EchoStream)

	// Create HTTP handler
	handler, err := rpc.NewHandler(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Support HTTP/2 and HTTP/1.1
	h2s := &http2.Server{}
	server := &http.Server{
		Addr:    ":8080",
		Handler: h2c.NewHandler(handler, h2s),
	}

	log.Println("Hyperway benchmark server starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
