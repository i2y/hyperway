package gateway

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/i2y/hyperway/proto"
)

// serveProtoExport handles proto file export requests.
func (g *Gateway) serveProtoExport(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the requested path
	requestPath := r.URL.Path

	// Handle different proto export endpoints
	switch {
	case requestPath == "/proto.zip":
		// Export all proto files as ZIP
		g.serveProtoZip(w, r)

	case requestPath == "/proto" || requestPath == "/proto/":
		// List available proto files
		g.listProtoFiles(w, r)

	case strings.HasPrefix(requestPath, "/proto/"):
		// Export specific proto file
		filename := strings.TrimPrefix(requestPath, "/proto/")
		g.serveProtoFile(w, r, filename)

	default:
		http.NotFound(w, r)
	}
}

// serveProtoZip exports all proto files as a ZIP archive.
func (g *Gateway) serveProtoZip(w http.ResponseWriter, _ *http.Request) {
	// Create exporter
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)

	// Export to ZIP
	zipData, err := exporter.ExportToZip(g.descriptor)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export proto files: %v", err), http.StatusInternalServerError)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"proto.zip\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Write ZIP data
	if _, err := w.Write(zipData); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

// listProtoFiles returns a list of available proto files.
func (g *Gateway) listProtoFiles(w http.ResponseWriter, _ *http.Request) {
	// Create exporter
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)

	// Export all files
	files, err := exporter.ExportFileDescriptorSet(g.descriptor)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list proto files: %v", err), http.StatusInternalServerError)
		return
	}

	// Build HTML response
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Proto Files</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 10px 0; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        .download-all { margin: 20px 0; font-weight: bold; }
    </style>
</head>
<body>
    <h1>Available Proto Files</h1>
    <div class="download-all">
        <a href="/proto.zip">📦 Download all as ZIP</a>
    </div>
    <ul>`

	for filename := range files {
		html += fmt.Sprintf(`
        <li>📄 <a href="/proto/%s">%s</a></li>`, filename, filename)
	}

	html += `
    </ul>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(html)); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

// serveProtoFile serves a specific proto file.
func (g *Gateway) serveProtoFile(w http.ResponseWriter, r *http.Request, filename string) {
	// Create exporter
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)

	// Export all files
	files, err := exporter.ExportFileDescriptorSet(g.descriptor)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export proto files: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the requested file
	content, ok := files[filename]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", path.Base(filename)))

	// Write proto content
	if _, err := w.Write([]byte(content)); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

// ExportProtos exports all proto files from the gateway.
// This is a programmatic API for exporting proto files.
func (g *Gateway) ExportProtos() (map[string]string, error) {
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)
	return exporter.ExportFileDescriptorSet(g.descriptor)
}

// ExportProtosToZip exports all proto files as a ZIP archive.
// This is a programmatic API for exporting proto files.
func (g *Gateway) ExportProtosToZip() ([]byte, error) {
	opts := proto.DefaultExportOptions()
	exporter := proto.NewExporter(&opts)
	return exporter.ExportToZip(g.descriptor)
}
