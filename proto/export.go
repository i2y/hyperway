// Package proto provides proto file export functionality.
package proto

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ExportOptions configures proto file export.
type ExportOptions struct {
	// IncludeComments adds comments to the exported proto files
	IncludeComments bool
	// SortElements sorts messages, fields, etc. alphabetically
	SortElements bool
	// Indent configures the indentation string (default: 2 spaces)
	Indent string
}

// DefaultExportOptions returns default export options.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		IncludeComments: true,
		SortElements:    false,
		Indent:          "  ",
	}
}

// Exporter handles proto file export operations.
type Exporter struct {
	options ExportOptions
	printer *protoprint.Printer
}

// NewExporter creates a new proto exporter.
func NewExporter(opts ExportOptions) *Exporter {
	printer := &protoprint.Printer{
		Compact:                      false,
		SortElements:                 opts.SortElements,
		Indent:                       opts.Indent,
		PreferMultiLineStyleComments: true,
	}

	return &Exporter{
		options: opts,
		printer: printer,
	}
}

// ExportFileDescriptorSet exports all proto files from a FileDescriptorSet.
func (e *Exporter) ExportFileDescriptorSet(fdset *descriptorpb.FileDescriptorSet) (map[string]string, error) {
	result := make(map[string]string)

	// Convert FileDescriptorProtos to desc.FileDescriptor
	files, err := desc.CreateFileDescriptors(fdset.File)
	if err != nil {
		return nil, fmt.Errorf("failed to create file descriptors: %w", err)
	}

	// Create a map of file descriptors by name for quick lookup
	fdMap := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, fdp := range fdset.File {
		if fdp.Name != nil {
			fdMap[*fdp.Name] = fdp
		}
	}

	// Export each file
	for _, fd := range files {
		var buf bytes.Buffer
		if err := e.printer.PrintProtoFile(fd, &buf); err != nil {
			return nil, fmt.Errorf("failed to export %s: %w", fd.GetName(), err)
		}
		content := buf.String()

		// Fix Editions syntax format if needed
		if fdp, ok := fdMap[fd.GetName()]; ok && fdp.Edition != nil {
			content = fixEditionsSyntax(content, fdp.Edition)
		}

		result[fd.GetName()] = content
	}

	return result, nil
}

// ExportFileDescriptorProto exports a single proto file.
func (e *Exporter) ExportFileDescriptorProto(fdp *descriptorpb.FileDescriptorProto) (string, error) {
	// Convert to desc.FileDescriptor
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return "", fmt.Errorf("failed to create file descriptor: %w", err)
	}

	var buf bytes.Buffer
	if err := e.printer.PrintProtoFile(fd, &buf); err != nil {
		return "", fmt.Errorf("failed to export proto: %w", err)
	}

	content := buf.String()

	// Fix Editions syntax format if needed
	if fdp.Edition != nil {
		content = fixEditionsSyntax(content, fdp.Edition)
	}

	return content, nil
}

// ExportToZip exports all proto files to a ZIP archive.
func (e *Exporter) ExportToZip(fdset *descriptorpb.FileDescriptorSet) ([]byte, error) {
	// Export all files
	files, err := e.ExportFileDescriptorSet(fdset)
	if err != nil {
		return nil, err
	}

	// Create ZIP archive
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Sort file names for consistent output
	fileNames := make([]string, 0, len(files))
	for name := range files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	// Add each file to ZIP
	for _, name := range fileNames {
		f, err := w.Create(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create ZIP entry %s: %w", name, err)
		}

		if _, err := io.WriteString(f, files[name]); err != nil {
			return nil, fmt.Errorf("failed to write ZIP entry %s: %w", name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP: %w", err)
	}

	return buf.Bytes(), nil
}

// ConvertToFileDescriptor converts a FileDescriptorProto to desc.FileDescriptor.
// This is a helper function for cases where you need the intermediate representation.
func ConvertToFileDescriptor(fdp *descriptorpb.FileDescriptorProto) (*desc.FileDescriptor, error) {
	return desc.CreateFileDescriptor(fdp)
}

// ConvertFromRegistry converts from protoreflect.FileDescriptor to FileDescriptorProto.
// This is useful when working with protobuf registry files.
func ConvertFromRegistry(file protoreflect.FileDescriptor) *descriptorpb.FileDescriptorProto {
	return protodesc.ToFileDescriptorProto(file)
}

// MergeFileDescriptorSets merges multiple FileDescriptorSets into one.
func MergeFileDescriptorSets(sets ...*descriptorpb.FileDescriptorSet) *descriptorpb.FileDescriptorSet {
	merged := &descriptorpb.FileDescriptorSet{}
	seen := make(map[string]bool)

	for _, set := range sets {
		if set == nil {
			continue
		}

		for _, file := range set.File {
			if file.Name != nil && !seen[*file.Name] {
				seen[*file.Name] = true
				merged.File = append(merged.File, proto.Clone(file).(*descriptorpb.FileDescriptorProto))
			}
		}
	}

	return merged
}

// fixEditionsSyntax fixes the Protobuf Editions syntax format in the exported proto content.
// The protoreflect/protoprint library outputs 'syntax = "editions";' but according to the
// official Protobuf Editions specification, it should be 'edition = "2023";' instead.
func fixEditionsSyntax(content string, edition *descriptorpb.Edition) string {
	if edition == nil {
		return content
	}

	// Convert edition enum to year string
	var editionYear string
	switch *edition {
	case descriptorpb.Edition_EDITION_2023:
		editionYear = "2023"
	case descriptorpb.Edition_EDITION_2024:
		editionYear = "2024"
	case descriptorpb.Edition_EDITION_UNKNOWN,
		descriptorpb.Edition_EDITION_LEGACY,
		descriptorpb.Edition_EDITION_PROTO2,
		descriptorpb.Edition_EDITION_PROTO3,
		descriptorpb.Edition_EDITION_1_TEST_ONLY,
		descriptorpb.Edition_EDITION_2_TEST_ONLY,
		descriptorpb.Edition_EDITION_99997_TEST_ONLY,
		descriptorpb.Edition_EDITION_99998_TEST_ONLY,
		descriptorpb.Edition_EDITION_99999_TEST_ONLY,
		descriptorpb.Edition_EDITION_MAX:
		// For non-editions or test editions, just return the original content
		return content
	default:
		// For any other unknown editions, just return the original content
		return content
	}

	// Replace 'syntax = "editions";' with 'edition = "2023";'
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == `syntax = "editions";` {
			lines[i] = fmt.Sprintf("edition = %q;", editionYear)
			break
		}
	}

	return strings.Join(lines, "\n")
}
