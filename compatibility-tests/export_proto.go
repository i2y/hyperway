//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	compatibility "github.com/i2y/hyperway/compatibility-tests"
	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/rpc"
)

func main() {
	// Create output directory
	outputDir := "proto"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create service
	svc := rpc.NewService("CompatibilityService",
		rpc.WithPackage("compatibility.v1"),
	)

	// Register all methods
	if err := rpc.Register(svc, "SimpleEcho", compatibility.SimpleEcho); err != nil {
		log.Fatalf("Failed to register SimpleEcho: %v", err)
	}
	if err := rpc.Register(svc, "ComplexEcho", compatibility.ComplexEcho); err != nil {
		log.Fatalf("Failed to register ComplexEcho: %v", err)
	}
	if err := rpc.Register(svc, "WellKnownEcho", compatibility.WellKnownEcho); err != nil {
		log.Fatalf("Failed to register WellKnownEcho: %v", err)
	}
	if err := rpc.Register(svc, "TestError", compatibility.TestError); err != nil {
		log.Fatalf("Failed to register TestError: %v", err)
	}
	if err := rpc.RegisterServerStream[compatibility.StreamRequest, compatibility.StreamResponse](svc, "ServerStream",
		func(ctx context.Context, req *compatibility.StreamRequest, stream rpc.ServerStream[compatibility.StreamResponse]) error {
			return compatibility.ServerStream(ctx, req, stream)
		}); err != nil {
		log.Fatalf("Failed to register ServerStream: %v", err)
	}

	// Export with Go package option for Connect-go
	options := []proto.ExportOption{
		proto.WithGoPackage("github.com/i2y/hyperway/compatibility-tests/gen;compatibility"),
	}

	files, err := svc.ExportAllProtosWithOptions(options...)
	if err != nil {
		log.Fatalf("Failed to export protos: %v", err)
	}

	// Write files
	for filename, content := range files {
		fullPath := filepath.Join(outputDir, filename)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			log.Fatalf("Failed to write file %s: %v", fullPath, err)
		}

		fmt.Printf("Generated: %s\n", fullPath)
	}

	fmt.Printf("\nSuccessfully exported %d proto files to %s/\n", len(files), outputDir)
	fmt.Println("\nNext steps to use with Connect-go:")
	fmt.Println("1. Install buf: brew install buf")
	fmt.Println("2. Create buf.gen.yaml with Connect-go plugins")
	fmt.Println("3. Run: buf generate proto")
	fmt.Println("4. Use generated Go code with Connect-go client")
}
