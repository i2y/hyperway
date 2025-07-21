// Package commands implements CLI commands for hyperway.
package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/grpcreflect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/descriptorpb"

	hyperwayproto "github.com/i2y/hyperway/proto"
)

// NewProtoCommand creates the proto command with subcommands.
func NewProtoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proto",
		Short: "Proto file management commands",
		Long:  "Commands for exporting and generating proto files from services and Go code.",
	}

	cmd.AddCommand(
		newProtoExportCommand(),
		newProtoGenerateCommand(),
	)

	return cmd
}

// protoExportOptions holds options for the proto export command.
type protoExportOptions struct {
	endpoint        string
	output          string
	format          string
	includeComments bool
	sortElements    bool
	timeout         time.Duration
}

func newProtoExportCommand() *cobra.Command {
	opts := &protoExportOptions{}

	cmd := &cobra.Command{
		Use:   "export [flags]",
		Short: "Export proto files from a running service",
		Long: `Export proto files from a running hyperway service using reflection.

The command connects to a service endpoint and exports all available proto definitions.
It supports exporting to individual files or a ZIP archive.

Examples:
  # Export to current directory
  hyperway proto export --endpoint http://localhost:8080

  # Export to specific directory
  hyperway proto export --endpoint http://localhost:8080 --output ./protos

  # Export as ZIP archive
  hyperway proto export --endpoint http://localhost:8080 --format zip --output service.zip

  # Export without comments and sorted
  hyperway proto export --endpoint http://localhost:8080 --no-comments --sort`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProtoExport(opts)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&opts.endpoint, "endpoint", "e", "http://localhost:8080", "Service endpoint URL")
	cmd.Flags().StringVarP(&opts.output, "output", "o", ".", "Output directory or file (for ZIP)")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "files", "Output format: files or zip")
	cmd.Flags().BoolVar(&opts.includeComments, "comments", true, "Include comments in proto files")
	cmd.Flags().BoolVar(&opts.sortElements, "sort", false, "Sort proto elements alphabetically")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 30*time.Second, "Request timeout")

	return cmd
}

func runProtoExport(opts *protoExportOptions) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: opts.timeout,
	}

	// Create reflection client
	reflectClient := grpcreflect.NewClient(client, opts.endpoint)

	// Create a new stream
	ctx := context.Background()
	stream := reflectClient.NewStream(ctx)
	defer stream.Close()

	// List services
	services, err := stream.ListServices()
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if len(services) == 0 {
		return fmt.Errorf("no services found at %s", opts.endpoint)
	}

	fmt.Printf("Found %d services at %s\n", len(services), opts.endpoint)

	// Create file descriptor set
	fdset := &descriptorpb.FileDescriptorSet{}
	seenFiles := make(map[string]bool)

	// Get file descriptors for all services
	for _, service := range services {
		fmt.Printf("Fetching descriptors for service: %s\n", service)

		// Get file containing the service
		fileDescriptors, err := stream.FileContainingSymbol(service)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get descriptor for %s: %v\n", service, err)
			continue
		}

		// Add file descriptors
		for _, fd := range fileDescriptors {
			// Skip if already seen
			if fd.Name != nil && seenFiles[*fd.Name] {
				continue
			}
			if fd.Name != nil {
				seenFiles[*fd.Name] = true
			}

			fdset.File = append(fdset.File, fd)
		}
	}

	if len(fdset.File) == 0 {
		return fmt.Errorf("no proto files could be exported")
	}

	// Create exporter
	exportOpts := hyperwayproto.ExportOptions{
		IncludeComments: opts.includeComments,
		SortElements:    opts.sortElements,
		Indent:          "  ",
	}
	exporter := hyperwayproto.NewExporter(exportOpts)

	// Export based on format
	switch opts.format {
	case "zip":
		return exportToZip(exporter, fdset, opts.output)
	case "files":
		return exportToFiles(exporter, fdset, opts.output)
	default:
		return fmt.Errorf("unknown format: %s", opts.format)
	}
}

func exportToZip(exporter *hyperwayproto.Exporter, fdset *descriptorpb.FileDescriptorSet, output string) error {
	// Export to ZIP
	zipData, err := exporter.ExportToZip(fdset)
	if err != nil {
		return fmt.Errorf("failed to create ZIP: %w", err)
	}

	// Determine output file
	outputFile := output
	if !strings.HasSuffix(outputFile, ".zip") {
		if output == "." {
			outputFile = "proto_export.zip"
		} else {
			outputFile = filepath.Join(output, "proto_export.zip")
		}
	}

	// Write ZIP file
	if err := os.WriteFile(outputFile, zipData, 0600); err != nil {
		return fmt.Errorf("failed to write ZIP file: %w", err)
	}

	fmt.Printf("Exported %d proto files to %s\n", len(fdset.File), outputFile)
	return nil
}

func exportToFiles(exporter *hyperwayproto.Exporter, fdset *descriptorpb.FileDescriptorSet, output string) error {
	// Export all files
	files, err := exporter.ExportFileDescriptorSet(fdset)
	if err != nil {
		return fmt.Errorf("failed to export files: %w", err)
	}

	// Create output directory if needed
	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write each file
	for filename, content := range files {
		outputPath := filepath.Join(output, filename)

		// Create subdirectories if needed
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write file
		if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		fmt.Printf("Exported: %s\n", outputPath)
	}

	fmt.Printf("\nSuccessfully exported %d proto files to %s\n", len(files), output)
	return nil
}

// protoGenerateOptions holds options for the proto generate command.
type protoGenerateOptions struct {
	input           string
	output          string
	packages        []string
	includeComments bool
	sortElements    bool
	recursive       bool
}

func newProtoGenerateCommand() *cobra.Command {
	opts := &protoGenerateOptions{}

	cmd := &cobra.Command{
		Use:   "generate [flags]",
		Short: "Generate proto files from Go source code",
		Long: `Generate proto files from Go structs and interfaces.

This command analyzes Go source code and generates corresponding proto files
based on struct definitions and RPC method signatures.

Examples:
  # Generate from current directory
  hyperway proto generate

  # Generate from specific directory
  hyperway proto generate --input ./model

  # Generate for specific packages
  hyperway proto generate --packages model,api

  # Generate recursively
  hyperway proto generate --recursive --output ./protos`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProtoGenerate(opts)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&opts.input, "input", "i", ".", "Input directory containing Go source files")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "./proto", "Output directory for generated proto files")
	cmd.Flags().StringSliceVarP(&opts.packages, "packages", "p", []string{}, "Specific packages to generate (comma-separated)")
	cmd.Flags().BoolVar(&opts.includeComments, "comments", true, "Include Go comments in proto files")
	cmd.Flags().BoolVar(&opts.sortElements, "sort", false, "Sort proto elements alphabetically")
	cmd.Flags().BoolVarP(&opts.recursive, "recursive", "r", false, "Process directories recursively")

	return cmd
}

func runProtoGenerate(opts *protoGenerateOptions) error {
	// TODO: Implement Go source to proto generation
	// This would require:
	// 1. Parsing Go source files using go/ast
	// 2. Analyzing struct definitions
	// 3. Converting Go types to proto types
	// 4. Generating proto files

	fmt.Println("Proto generation from Go source is not yet implemented.")
	fmt.Println("This feature will analyze Go structs and generate corresponding proto files.")
	fmt.Println("\nPlanned features:")
	fmt.Println("- Convert Go structs to proto messages")
	fmt.Println("- Convert interface methods to RPC services")
	fmt.Println("- Handle Go tags for proto options")
	fmt.Println("- Support for nested types and imports")

	return nil
}
