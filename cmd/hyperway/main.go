// Package main provides the hyperway CLI tool for managing proto files and services.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/i2y/hyperway/cmd/hyperway/commands"
)

var (
	// Version information (set by build flags)
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "hyperway",
		Short: "High-performance RPC framework with dynamic proto generation",
		Long: `Hyperway is a Go RPC library that eliminates the need for manual .proto files 
by generating Protobuf schemas dynamically at runtime from Go structs.

It provides tools for exporting proto files, generating schemas, and managing services.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate),
	}

	// Add commands
	rootCmd.AddCommand(
		commands.NewProtoCommand(),
		commands.NewVersionCommand(version, commit, buildDate),
		commands.NewServeCommand(),
	)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
