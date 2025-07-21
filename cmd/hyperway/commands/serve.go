package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// serveOptions holds options for the serve command.
type serveOptions struct {
	port             int
	host             string
	configFile       string
	enableReflection bool
	enableOpenAPI    bool
	enableMetrics    bool
	gracefulTimeout  time.Duration
}

// NewServeCommand creates the serve command.
func NewServeCommand() *cobra.Command {
	opts := &serveOptions{}

	cmd := &cobra.Command{
		Use:   "serve [flags]",
		Short: "Start a hyperway RPC server",
		Long: `Start a hyperway RPC server with the specified configuration.

This command starts an HTTP server that can handle gRPC, Connect, and REST protocols
simultaneously.

Examples:
  # Start server on default port
  hyperway serve

  # Start server on specific port
  hyperway serve --port 9090

  # Start with configuration file
  hyperway serve --config server.yaml

  # Enable all features
  hyperway serve --reflection --openapi --metrics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(opts)
		},
	}

	// Add flags
	cmd.Flags().IntVarP(&opts.port, "port", "p", 8080, "Server port")
	cmd.Flags().StringVar(&opts.host, "host", "0.0.0.0", "Server host")
	cmd.Flags().StringVarP(&opts.configFile, "config", "c", "", "Configuration file path")
	cmd.Flags().BoolVar(&opts.enableReflection, "reflection", true, "Enable gRPC reflection")
	cmd.Flags().BoolVar(&opts.enableOpenAPI, "openapi", true, "Enable OpenAPI endpoint")
	cmd.Flags().BoolVar(&opts.enableMetrics, "metrics", false, "Enable metrics endpoint")
	cmd.Flags().DurationVar(&opts.gracefulTimeout, "graceful-timeout", 30*time.Second, "Graceful shutdown timeout")

	return cmd
}

func runServe(opts *serveOptions) error {
	// TODO: Implement actual server startup
	// This would require:
	// 1. Loading configuration
	// 2. Setting up services
	// 3. Configuring middleware
	// 4. Starting HTTP server

	fmt.Printf("Starting hyperway server...\n")
	fmt.Printf("Host: %s\n", opts.host)
	fmt.Printf("Port: %d\n", opts.port)
	fmt.Printf("Reflection: %v\n", opts.enableReflection)
	fmt.Printf("OpenAPI: %v\n", opts.enableOpenAPI)
	fmt.Printf("Metrics: %v\n", opts.enableMetrics)

	if opts.configFile != "" {
		fmt.Printf("Config file: %s\n", opts.configFile)
	}

	// For now, just create a simple HTTP server
	mux := http.NewServeMux()

	// Add a health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// Add placeholder endpoints
	if opts.enableOpenAPI {
		mux.HandleFunc("/openapi", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"info":{"title":"Hyperway API","version":"1.0.0"}}`)
		})
	}

	if opts.enableMetrics {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintln(w, "# HELP hyperway_requests_total Total number of requests")
			fmt.Fprintln(w, "# TYPE hyperway_requests_total counter")
			fmt.Fprintln(w, "hyperway_requests_total 0")
		})
	}

	// Create server
	addr := fmt.Sprintf("%s:%d", opts.host, opts.port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("\nServer listening on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), opts.gracefulTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	fmt.Println("Server stopped")
	return nil
}
