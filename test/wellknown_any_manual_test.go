package test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/i2y/hyperway/rpc"
)

// Test Any type with manual creation
func TestAnyManual(t *testing.T) {
	type AnyManualRequest struct {
		Data *anypb.Any `json:"data"`
		Name string     `json:"name"`
	}

	type AnyManualResponse struct {
		TypeURL   string `json:"type_url"`
		HasData   bool   `json:"has_data"`
		DataBytes int    `json:"data_bytes"`
	}

	// Create service
	svc := rpc.NewService("AnyManualService", rpc.WithPackage("test.v1"))

	// Handler
	handler := func(ctx context.Context, req *AnyManualRequest) (*AnyManualResponse, error) {
		resp := &AnyManualResponse{
			HasData: req.Data != nil,
		}
		if req.Data != nil {
			resp.TypeURL = req.Data.TypeUrl
			resp.DataBytes = len(req.Data.Value)
		}
		return resp, nil
	}

	// Register handler
	if err := rpc.Register(svc, "TestAnyManual", handler); err != nil {
		t.Fatal(err)
	}

	// Create Any containing a StringValue
	stringValue := &wrapperspb.StringValue{Value: "hello world"}
	anyData, err := anypb.New(stringValue)
	if err != nil {
		t.Fatal(err)
	}

	// Test with manually created request
	req := &AnyManualRequest{
		Data: anyData,
		Name: "test",
	}

	// Call handler directly (bypassing JSON serialization)
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify response
	if !resp.HasData {
		t.Error("Expected has_data=true")
	}

	expectedTypeURL := "type.googleapis.com/google.protobuf.StringValue"
	if resp.TypeURL != expectedTypeURL {
		t.Errorf("Expected type_url=%s, got %s", expectedTypeURL, resp.TypeURL)
	}

	if resp.DataBytes == 0 {
		t.Error("Expected non-zero data_bytes")
	}

	t.Logf("Any test passed: type_url=%s, data_bytes=%d", resp.TypeURL, resp.DataBytes)
}
