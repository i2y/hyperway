package rpc

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/i2y/hyperway/schema"
)

func TestServiceWithEditions(t *testing.T) { //nolint:gocyclo // This is a comprehensive test suite
	t.Run("Proto3 service (default)", func(t *testing.T) {
		svc := NewService("TestService")

		// Register a simple method
		type Input struct {
			Name string `json:"name"`
		}
		type Output struct {
			Message string `json:"message"`
		}

		handler := func(ctx context.Context, in *Input) (*Output, error) {
			return &Output{Message: "Hello " + in.Name}, nil
		}

		method := NewMethod("Greet", handler)
		err := svc.Register(method.Build())
		if err != nil {
			t.Fatalf("Failed to register method: %v", err)
		}

		// Get the file descriptor set
		fdset := svc.GetFileDescriptorSet()
		if fdset == nil || len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		// Find the service file
		var serviceFile *descriptorpb.FileDescriptorProto
		for _, file := range fdset.File {
			if len(file.Service) > 0 {
				serviceFile = file
				break
			}
		}

		if serviceFile == nil {
			t.Fatal("Service file not found")
		}

		if serviceFile.GetSyntax() != "proto3" {
			t.Errorf("Service file syntax = %q, want %q", serviceFile.GetSyntax(), "proto3")
		}

		if serviceFile.Edition != nil {
			t.Error("Proto3 service file should not have Edition field set")
		}
	})

	t.Run("Editions 2023 service", func(t *testing.T) {
		svc := NewService("TestService", WithEdition(schema.Edition2023))

		// Register a simple method
		type Input struct {
			Name string `json:"name"`
		}
		type Output struct {
			Message string `json:"message"`
		}

		handler := func(ctx context.Context, in *Input) (*Output, error) {
			return &Output{Message: "Hello " + in.Name}, nil
		}

		method := NewMethod("Greet", handler)
		err := svc.Register(method.Build())
		if err != nil {
			t.Fatalf("Failed to register method: %v", err)
		}

		// Get the file descriptor set
		fdset := svc.GetFileDescriptorSet()
		if fdset == nil || len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		// Find the service file
		var serviceFile *descriptorpb.FileDescriptorProto
		for _, file := range fdset.File {
			if len(file.Service) > 0 {
				serviceFile = file
				break
			}
		}

		if serviceFile == nil {
			t.Fatal("Service file not found")
		}

		if serviceFile.GetSyntax() != "editions" {
			t.Errorf("Service file syntax = %q, want %q", serviceFile.GetSyntax(), "editions")
		}

		if serviceFile.Edition == nil {
			t.Fatal("Editions service file should have Edition field set")
		}

		if *serviceFile.Edition != descriptorpb.Edition_EDITION_2023 {
			t.Errorf("Service file edition = %v, want %v", *serviceFile.Edition, descriptorpb.Edition_EDITION_2023)
		}
	})

	t.Run("Editions 2024 service", func(t *testing.T) {
		svc := NewService("TestService", WithEdition(schema.Edition2024))

		if !svc.options.UseEditions {
			t.Error("WithEdition should enable UseEditions")
		}

		if svc.options.Edition != schema.Edition2024 {
			t.Errorf("Service edition = %q, want %q", svc.options.Edition, schema.Edition2024)
		}

		// Register a method to generate file descriptor
		type Input struct{}
		type Output struct{}

		handler := func(ctx context.Context, in *Input) (*Output, error) {
			return &Output{}, nil
		}

		method := NewMethod("Test", handler)
		err := svc.Register(method.Build())
		if err != nil {
			// EDITION_2024 might not be supported yet by the Go protobuf runtime
			if testing.Verbose() {
				t.Logf("Register with EDITION_2024 failed (this might be expected): %v", err)
			}
			t.Skip("EDITION_2024 not yet supported by Go protobuf runtime")
		}

		// Get the file descriptor set
		fdset := svc.GetFileDescriptorSet()
		if fdset == nil || len(fdset.File) == 0 {
			t.Fatal("No files in FileDescriptorSet")
		}

		// Find the service file
		var serviceFile *descriptorpb.FileDescriptorProto
		for _, file := range fdset.File {
			if len(file.Service) > 0 {
				serviceFile = file
				break
			}
		}

		if serviceFile == nil {
			t.Fatal("Service file not found")
		}

		if serviceFile.Edition == nil {
			t.Fatal("Editions service file should have Edition field set")
		}

		if *serviceFile.Edition != descriptorpb.Edition_EDITION_2024 {
			t.Errorf("Service file edition = %v, want %v", *serviceFile.Edition, descriptorpb.Edition_EDITION_2024)
		}
	})

	t.Run("Builder cache with editions", func(t *testing.T) {
		// Create two services with different editions but same package
		svc1 := NewService("TestService", WithPackage("test.v1"))
		svc2 := NewService("TestService", WithPackage("test.v1"), WithEdition(schema.Edition2023))

		// They should have different builders
		if svc1.builder == svc2.builder {
			t.Error("Services with different syntax modes should have different builders")
		}

		// Create another service with same edition
		svc3 := NewService("TestService", WithPackage("test.v1"), WithEdition(schema.Edition2023))

		// svc2 and svc3 should share the same builder
		if svc2.builder != svc3.builder {
			t.Error("Services with same package and edition should share the same builder")
		}
	})

	t.Run("WithEdition option", func(t *testing.T) {
		opts := &ServiceOptions{}
		WithEdition("2024")(opts)

		if !opts.UseEditions {
			t.Error("WithEdition should set UseEditions to true")
		}

		if opts.Edition != "2024" {
			t.Errorf("Edition = %q, want %q", opts.Edition, "2024")
		}
	})
}
