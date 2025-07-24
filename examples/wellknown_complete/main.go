package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/i2y/hyperway/rpc"
)

// CompleteRequest demonstrates all Well-Known Types
type CompleteRequest struct {
	// Basic Well-Known Types
	Timestamp *timestamppb.Timestamp `json:"timestamp"`                // time.Time can also be used
	Empty     struct{}               `json:"empty"`                    // google.protobuf.Empty
	StringTag string                 `json:"string_tag" proto:"empty"` // Tagged as Empty

	// Complex Well-Known Types
	Config     *structpb.Struct       `json:"config"`      // Dynamic JSON structure
	Value      *structpb.Value        `json:"value"`       // Any JSON value
	List       *structpb.ListValue    `json:"list"`        // Mixed type list
	UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"` // Field paths
	AnyData    *anypb.Any             `json:"any_data"`    // Any protobuf message

	// Map with dynamic values
	Properties map[string]*structpb.Value `json:"properties"`
}

type CompleteResponse struct {
	Success  bool   `json:"success"`
	Summary  string `json:"summary"`
	TypeInfo string `json:"type_info"`
}

// Service implementation
type CompleteService struct{}

func (s *CompleteService) ProcessComplete(ctx context.Context, req *CompleteRequest) (*CompleteResponse, error) {
	summary := "Processed: "

	// Check timestamp
	if req.Timestamp != nil {
		summary += fmt.Sprintf("timestamp=%v, ", req.Timestamp.AsTime().Format(time.RFC3339))
	}

	// Check config
	if req.Config != nil {
		summary += fmt.Sprintf("config_fields=%d, ", len(req.Config.Fields))
	}

	// Check value type
	if req.Value != nil {
		switch v := req.Value.Kind.(type) {
		case *structpb.Value_StringValue:
			summary += fmt.Sprintf("value_type=string(%s), ", v.StringValue)
		case *structpb.Value_NumberValue:
			summary += fmt.Sprintf("value_type=number(%f), ", v.NumberValue)
		case *structpb.Value_BoolValue:
			summary += fmt.Sprintf("value_type=bool(%t), ", v.BoolValue)
		default:
			summary += "value_type=other, "
		}
	}

	// Check list
	if req.List != nil {
		summary += fmt.Sprintf("list_items=%d, ", len(req.List.Values))
	}

	// Check field mask
	if req.UpdateMask != nil {
		summary += fmt.Sprintf("update_paths=%v, ", req.UpdateMask.Paths)
	}

	// Check properties
	summary += fmt.Sprintf("properties=%d", len(req.Properties))

	// Type info for Any
	typeInfo := "no any data"
	if req.AnyData != nil {
		typeInfo = fmt.Sprintf("any_type=%s", req.AnyData.TypeUrl)
	}

	return &CompleteResponse{
		Success:  true,
		Summary:  summary,
		TypeInfo: typeInfo,
	}, nil
}

// Demonstrate creating Any from various types
func (s *CompleteService) CreateAny(ctx context.Context, req *struct{}) (*anypb.Any, error) {
	// Create a StringValue and pack it into Any
	str := &wrapperspb.StringValue{Value: "Hello from Any!"}
	any, err := anypb.New(str)
	if err != nil {
		return nil, err
	}
	return any, nil
}

func main() {
	// Create service
	svc := rpc.NewService("CompleteService",
		rpc.WithPackage("complete.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Create service instance
	completeService := &CompleteService{}

	// Register methods
	if err := rpc.Register(svc, "ProcessComplete", completeService.ProcessComplete); err != nil {
		log.Fatal(err)
	}
	if err := rpc.Register(svc, "CreateAny", completeService.CreateAny); err != nil {
		log.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	log.Println("Complete Well-Known Types example server running on :8080")
	log.Println("Test endpoints:")
	log.Println("- POST http://localhost:8080/complete.v1.CompleteService/ProcessComplete")
	log.Println("- POST http://localhost:8080/complete.v1.CompleteService/CreateAny")
	log.Fatal(http.ListenAndServe(":8080", gateway))
}
