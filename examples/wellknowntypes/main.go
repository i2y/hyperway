package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/i2y/hyperway/rpc"
)

// ConfigUpdateRequest demonstrates using FieldMask for partial updates
type ConfigUpdateRequest struct {
	// Dynamic configuration using Struct
	Config *structpb.Struct `json:"config"`
	// FieldMask to specify which fields to update
	UpdateMask *fieldmaskpb.FieldMask `json:"update_mask"`
}

type ConfigUpdateResponse struct {
	Success bool             `json:"success"`
	Updated *structpb.Struct `json:"updated"`
}

// FlexibleDataRequest demonstrates using Value and ListValue
type FlexibleDataRequest struct {
	// Can be string, number, bool, null, list, or struct
	SingleValue *structpb.Value `json:"single_value"`
	// List of mixed types
	MixedList *structpb.ListValue `json:"mixed_list"`
	// Map with dynamic values
	Properties map[string]*structpb.Value `json:"properties"`
}

type FlexibleDataResponse struct {
	Processed bool   `json:"processed"`
	Summary   string `json:"summary"`
}

// Service implementation
type ConfigService struct{}

func (s *ConfigService) UpdateConfig(ctx context.Context, req *ConfigUpdateRequest) (*ConfigUpdateResponse, error) {
	log.Printf("UpdateConfig called")
	log.Printf("Request type: %T", req)

	// Safely handle nil UpdateMask
	if req.UpdateMask != nil {
		log.Printf("UpdateMask paths: %v", req.UpdateMask.GetPaths())
		log.Printf("UpdateMask type: %T", req.UpdateMask)
	} else {
		log.Printf("UpdateMask is nil")
	}

	// Safely handle nil Config
	if req.Config != nil {
		log.Printf("Config has %d fields", len(req.Config.Fields))
		log.Printf("Config type: %T", req.Config)
		for k, v := range req.Config.Fields {
			log.Printf("  %s: %v (type: %T)", k, v, v)
		}
	} else {
		log.Printf("Config is nil")
	}

	// In a real implementation, you would:
	// 1. Load existing config
	// 2. Apply updates only for fields specified in UpdateMask
	// 3. Save and return updated config

	// For demo, just echo back the config
	return &ConfigUpdateResponse{
		Success: true,
		Updated: req.Config,
	}, nil
}

func (s *ConfigService) ProcessFlexibleData(ctx context.Context, req *FlexibleDataRequest) (*FlexibleDataResponse, error) {
	log.Printf("ProcessFlexibleData called")

	propertyCount := 0
	if req.Properties != nil {
		propertyCount = len(req.Properties)
	}
	summary := fmt.Sprintf("Received data with %d properties", propertyCount)

	// Process single value
	if req.SingleValue != nil && req.SingleValue.Kind != nil {
		switch v := req.SingleValue.Kind.(type) {
		case *structpb.Value_StringValue:
			summary += fmt.Sprintf(", single value is string: %s", v.StringValue)
		case *structpb.Value_NumberValue:
			summary += fmt.Sprintf(", single value is number: %f", v.NumberValue)
		case *structpb.Value_BoolValue:
			summary += fmt.Sprintf(", single value is bool: %t", v.BoolValue)
		case *structpb.Value_StructValue:
			if v.StructValue != nil {
				summary += fmt.Sprintf(", single value is struct with %d fields", len(v.StructValue.Fields))
			}
		}
	}

	// Process list
	if req.MixedList != nil && req.MixedList.Values != nil {
		summary += fmt.Sprintf(", list has %d items", len(req.MixedList.Values))
	}

	log.Printf("ProcessFlexibleData: %s", summary)

	return &FlexibleDataResponse{
		Processed: true,
		Summary:   summary,
	}, nil
}

func main() {
	// Create service
	svc := rpc.NewService("ConfigService",
		rpc.WithPackage("config.v1"),
		rpc.WithValidation(true),
		rpc.WithReflection(true),
	)

	// Create service instance
	configService := &ConfigService{}

	// Register methods
	if err := rpc.Register(svc, "UpdateConfig", configService.UpdateConfig); err != nil {
		log.Fatal(err)
	}
	if err := rpc.Register(svc, "ProcessFlexibleData", configService.ProcessFlexibleData); err != nil {
		log.Fatal(err)
	}

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	// Add a test endpoint that demonstrates usage
	mux.HandleFunc("/test", testEndpoint)

	// Create HTTP/2 server with h2c support
	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	log.Println("Well-Known Types example server running on :8080")
	log.Println("- gRPC/Connect: :8080 (with HTTP/2 h2c support)")
	log.Println("- Test endpoint: http://localhost:8080/test")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func testEndpoint(w http.ResponseWriter, r *http.Request) {
	examples := []struct {
		name string
		data any
	}{
		{
			name: "UpdateConfig with FieldMask",
			data: map[string]any{
				"config": map[string]any{
					"theme":    "dark",
					"language": "ja",
					"notifications": map[string]any{
						"email": true,
						"push":  false,
					},
				},
				"update_mask": map[string]any{
					"paths": []string{"theme", "notifications.email"},
				},
			},
		},
		{
			name: "FlexibleData with mixed types",
			data: map[string]any{
				"single_value": "Hello, World!",
				"mixed_list": map[string]any{
					"values": []any{
						map[string]any{"string_value": "text"},
						map[string]any{"number_value": 42.5},
						map[string]any{"bool_value": true},
					},
				},
				"properties": map[string]any{
					"name":   map[string]any{"string_value": "test"},
					"count":  map[string]any{"number_value": 100},
					"active": map[string]any{"bool_value": true},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(examples)
}
