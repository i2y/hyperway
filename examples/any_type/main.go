package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/i2y/hyperway/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// AnyTestRequest demonstrates using Any types in requests
type AnyTestRequest struct {
	Name    string     `json:"name"`
	Details *anypb.Any `json:"details"`
}

// AnyTestResponse demonstrates using Any types in responses
type AnyTestResponse struct {
	Message string     `json:"message"`
	Result  *anypb.Any `json:"result"`
}

// TestService demonstrates Any type handling
type TestService struct{}

// ProcessAny processes a request with Any type
func (s *TestService) ProcessAny(ctx context.Context, req *AnyTestRequest) (*AnyTestResponse, error) {
	log.Printf("Received request with name=%s", req.Name)

	// Unpack the Any type to see what's inside
	if req.Details != nil {
		log.Printf("Details type_url: %s", req.Details.TypeUrl)

		// Try to unmarshal as different types
		var msg string
		switch req.Details.TypeUrl {
		case "type.googleapis.com/google.protobuf.StringValue":
			strVal := &wrapperspb.StringValue{}
			if err := req.Details.UnmarshalTo(strVal); err == nil {
				msg = fmt.Sprintf("Received string: %s", strVal.Value)
			}
		case "type.googleapis.com/google.protobuf.Timestamp":
			ts := &timestamppb.Timestamp{}
			if err := req.Details.UnmarshalTo(ts); err == nil {
				msg = fmt.Sprintf("Received timestamp: %v", ts.AsTime())
			}
		case "type.googleapis.com/google.protobuf.Struct":
			s := &structpb.Struct{}
			if err := req.Details.UnmarshalTo(s); err == nil {
				msg = fmt.Sprintf("Received struct with %d fields", len(s.Fields))
			}
		default:
			msg = fmt.Sprintf("Received unknown type: %s", req.Details.TypeUrl)
		}

		// Create response with Any type
		resultStr := wrapperspb.String("Processed: " + msg)
		resultAny, err := anypb.New(resultStr)
		if err != nil {
			return nil, fmt.Errorf("failed to create Any: %w", err)
		}

		return &AnyTestResponse{
			Message: msg,
			Result:  resultAny,
		}, nil
	}

	// Return empty Any if no details provided
	emptyAny, _ := anypb.New(wrapperspb.String("No details provided"))
	return &AnyTestResponse{
		Message: "No details provided",
		Result:  emptyAny,
	}, nil
}

func main() {
	// Create service
	svc := rpc.NewService("AnyTestService",
		rpc.WithPackage("example.anytest.v1"))

	// Create handler
	handler := &TestService{}

	// Register method
	err := rpc.RegisterMethod(svc,
		rpc.NewMethod("ProcessAny", handler.ProcessAny).
			In(AnyTestRequest{}).
			Out(AnyTestResponse{}))
	if err != nil {
		log.Fatalf("Failed to register method: %v", err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Start server
	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	log.Println("Starting Any type test server on :8080")
	log.Println("Service registered at: /example.anytest.v1.AnyTestService/ProcessAny")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
