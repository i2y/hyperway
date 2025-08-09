// Package main demonstrates how to export proto files from hyperway services.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/i2y/hyperway/proto"
	"github.com/i2y/hyperway/rpc"
)

// Constants for file permissions
const (
	dirPermission  = 0750
	filePermission = 0600
)

// Example types
type User struct {
	ID       string            `json:"id"`
	Name     string            `json:"name" validate:"required,min=3,max=50"`
	Email    string            `json:"email" validate:"required,email"`
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
	Profile  *Profile          `json:"profile,omitempty"`
}

type Profile struct {
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}

type CreateUserRequest struct {
	User *User `json:"user" validate:"required"`
}

type CreateUserResponse struct {
	Success bool   `json:"success"`
	UserID  string `json:"user_id"`
}

type GetUserRequest struct {
	ID string `json:"id" validate:"required"`
}

type GetUserResponse struct {
	User *User `json:"user"`
}

// Dummy handlers
func createUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	return &CreateUserResponse{Success: true, UserID: "user-123"}, nil
}

func getUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	return &GetUserResponse{User: &User{ID: req.ID, Name: "Example User"}}, nil
}

func exportSingleFile(svc *rpc.Service, output string) {
	protoContent, err := svc.ExportProto()
	if err != nil {
		log.Fatalf("Failed to export proto: %v", err)
	}

	if output != "" {
		// Write to file
		filename := filepath.Join(output, "service.proto")
		if err := os.MkdirAll(output, dirPermission); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		if err := os.WriteFile(filename, []byte(protoContent), filePermission); err != nil {
			log.Fatalf("Failed to write proto file: %v", err)
		}
		fmt.Printf("Exported proto to %s\n", filename)
	} else {
		// Write to stdout
		fmt.Println(protoContent)
	}
}

func exportAsZip(svc *rpc.Service, zipOutput string) {
	files, err := svc.ExportAllProtos()
	if err != nil {
		log.Fatalf("Failed to export protos: %v", err)
	}

	// Create FileDescriptorSet
	fdset := svc.GetFileDescriptorSet()

	// Export to ZIP
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)
	zipData, err := exporter.ExportToZip(fdset)
	if err != nil {
		log.Fatalf("Failed to create ZIP: %v", err)
	}

	// Write ZIP file
	if err := os.WriteFile(zipOutput, zipData, filePermission); err != nil {
		log.Fatalf("Failed to write ZIP file: %v", err)
	}
	fmt.Printf("Exported %d proto files to %s\n", len(files), zipOutput)
}

func exportAllFiles(svc *rpc.Service, output string) {
	files, err := svc.ExportAllProtos()
	if err != nil {
		log.Fatalf("Failed to export protos: %v", err)
	}

	if output != "" {
		// Write to directory
		if err := os.MkdirAll(output, dirPermission); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}

		for filename, content := range files {
			fullPath := filepath.Join(output, filename)
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, dirPermission); err != nil {
				log.Fatalf("Failed to create directory %s: %v", dir, err)
			}
			if err := os.WriteFile(fullPath, []byte(content), filePermission); err != nil {
				log.Fatalf("Failed to write file %s: %v", fullPath, err)
			}
			fmt.Printf("Exported %s\n", fullPath)
		}
		fmt.Printf("\nExported %d proto files to %s\n", len(files), output)
	} else {
		// Write to stdout
		for filename, content := range files {
			fmt.Printf("// File: %s\n", filename)
			fmt.Println(content)
		}
	}
}

func main() {
	var (
		output     = flag.String("output", "", "Output directory for proto files (default: stdout)")
		zipOutput  = flag.String("zip", "", "Export as ZIP file")
		singleFile = flag.Bool("single", false, "Export as single file (when possible)")
	)
	flag.Parse()

	// Create a sample service
	svc := rpc.NewService("UserService",
		rpc.WithPackage("example.user.v1"),
		rpc.WithValidation(true),
	)

	// Register methods
	if err := rpc.Register(svc, "CreateUser", createUser); err != nil {
		log.Fatalf("Failed to register CreateUser: %v", err)
	}
	if err := rpc.Register(svc, "GetUser", getUser); err != nil {
		log.Fatalf("Failed to register GetUser: %v", err)
	}

	// Export proto files
	switch {
	case *singleFile:
		exportSingleFile(svc, *output)
	case *zipOutput != "":
		exportAsZip(svc, *zipOutput)
	default:
		exportAllFiles(svc, *output)
	}
}

// Usage examples:
//
// Export to stdout:
//   go run examples/export-proto/main.go
//
// Export to directory:
//   go run examples/export-proto/main.go -output ./proto
//
// Export as single file:
//   go run examples/export-proto/main.go -single -output ./proto
//
// Export as ZIP:
//   go run examples/export-proto/main.go -zip proto.zip
