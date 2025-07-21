package gateway

import (
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/grpcreflect"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// descriptorResolver implements resolution for our dynamic descriptors.
type descriptorResolver struct {
	services []*Service
}

func (d *descriptorResolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	// Create a file registry to handle dependencies
	files := &protoregistry.Files{}

	// First, register all files
	for _, svc := range d.services {
		if svc.Descriptors != nil {
			for _, file := range svc.Descriptors.File {
				fd, err := protodesc.NewFile(file, files)
				if err == nil {
					if err := files.RegisterFile(fd); err != nil {
						return nil, fmt.Errorf("failed to register file %s: %w", fd.Path(), err)
					}
				}
			}
		}
	}

	// Then find the requested file
	fd, err := files.FindFileByPath(path)
	if err != nil {
		return nil, protoregistry.NotFound
	}
	return fd, nil
}

func (d *descriptorResolver) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	fmt.Printf("[DEBUG] FindDescriptorByName called with: %s\n", name)

	// First try the global registry
	if desc, err := protoregistry.GlobalFiles.FindDescriptorByName(name); err == nil {
		fmt.Printf("[DEBUG] Found in global registry\n")
		return desc, nil
	}

	// Create a file registry to handle dependencies
	files := &protoregistry.Files{}

	// First, register well-known types from the global registry
	// This ensures imports like google/protobuf/timestamp.proto are available
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if strings.HasPrefix(fd.Path(), "google/protobuf/") {
			if err := files.RegisterFile(fd); err != nil {
				fmt.Printf("[DEBUG] Failed to register well-known type %s: %v\n", fd.Path(), err)
			} else {
				fmt.Printf("[DEBUG] Registered well-known type: %s\n", fd.Path())
			}
		}
		return true
	})

	// Track which files we've already registered to avoid duplicates
	registeredFiles := make(map[string]bool)

	// Register all files from services
	for _, svc := range d.services {
		if svc.Descriptors != nil {
			fmt.Printf("[DEBUG] Service %s.%s has %d files\n", svc.Package, svc.Name, len(svc.Descriptors.File))
			for _, file := range svc.Descriptors.File {
				// Skip if already registered
				if registeredFiles[file.GetName()] {
					continue
				}

				fmt.Printf("[DEBUG] Registering file: %s (package: %s)\n", file.GetName(), file.GetPackage())
				if len(file.Service) > 0 {
					fmt.Printf("[DEBUG] File contains %d services:\n", len(file.Service))
					for _, s := range file.Service {
						fmt.Printf("[DEBUG]   - %s\n", s.GetName())
					}
				}

				fd, err := protodesc.NewFile(file, files)
				if err == nil {
					if err := files.RegisterFile(fd); err != nil {
						fmt.Printf("[DEBUG] Failed to register file: %v\n", err)
						// Continue on error to try other files
						continue
					}
					registeredFiles[file.GetName()] = true
					fmt.Printf("[DEBUG] Successfully registered file: %s\n", file.GetName())
				} else {
					fmt.Printf("[DEBUG] Failed to create file descriptor: %v\n", err)
				}
			}
		}
	}

	// Debug: list all registered files
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		fmt.Printf("[DEBUG] Registered file: %s\n", fd.Path())
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			fmt.Printf("[DEBUG]   Service: %s\n", svc.FullName())
		}
		return true
	})

	// Then find the descriptor
	desc, err := files.FindDescriptorByName(name)
	if err != nil {
		fmt.Printf("[DEBUG] Failed to find descriptor: %v\n", err)
		// Let's also check what descriptors ARE available
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			fmt.Printf("[DEBUG] Available file: %s\n", fd.Path())
			for i := 0; i < fd.Services().Len(); i++ {
				svc := fd.Services().Get(i)
				fmt.Printf("[DEBUG]   Service: %s\n", svc.FullName())
			}
			return true
		})
		return nil, protoregistry.NotFound
	}
	fmt.Printf("[DEBUG] Found descriptor successfully\n")
	return desc, nil
}

// CreateReflectionHandlers creates the reflection handlers for the gateway.
func (g *Gateway) CreateReflectionHandlers() (map[string]http.Handler, error) {
	if !g.options.EnableReflection {
		return nil, nil
	}

	// Simple namer that returns all service names
	namer := grpcreflect.NamerFunc(func() []string {
		var serviceNames []string
		for _, svc := range g.services {
			// Add the fully-qualified service name
			fullName := svc.Package + "." + svc.Name
			serviceNames = append(serviceNames, fullName)
		}
		return serviceNames
	})

	// Create resolver for our descriptors
	resolver := &descriptorResolver{services: g.services}

	// Create a reflector with our namer and resolver
	reflector := grpcreflect.NewReflector(namer, grpcreflect.WithDescriptorResolver(resolver))

	// Get the Connect handlers for reflection
	handlers := make(map[string]http.Handler)

	// v1 reflection
	v1Path, v1Handler := grpcreflect.NewHandlerV1(reflector)
	handlers[v1Path] = v1Handler

	// v1alpha reflection (for backward compatibility)
	v1alphaPath, v1alphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	handlers[v1alphaPath] = v1alphaHandler

	return handlers, nil
}
