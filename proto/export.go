// Package proto provides proto file export functionality.
package proto

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jhump/protoreflect/v2/protoprint"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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

	// Add Well-Known Types to FileDescriptorSet if they are referenced but not included
	fdset = e.addWellKnownTypes(fdset)

	// Convert FileDescriptorProtos to protoreflect.FileDescriptor
	files, err := protodesc.NewFiles(fdset)
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
	var exportErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		var buf bytes.Buffer
		if err := e.printer.PrintProtoFile(fd, &buf); err != nil {
			// Store error and stop iteration
			exportErr = fmt.Errorf("failed to export %s: %w", fd.Path(), err)
			return false
		}
		content := buf.String()

		// Fix Editions syntax format if needed
		if fdp, ok := fdMap[fd.Path()]; ok {
			if fdp.Edition != nil {
				content = fixEditionsSyntax(content, fdp.Edition)
			}
			// Fix proto3 optional fields
			content = fixProto3Optional(content, fdp)
		}

		result[fd.Path()] = content
		return true
	})

	if exportErr != nil {
		return nil, exportErr
	}

	return result, nil
}

// ExportFileDescriptorProto exports a single proto file.
func (e *Exporter) ExportFileDescriptorProto(fdp *descriptorpb.FileDescriptorProto) (string, error) {
	// Create a FileDescriptorSet with just this file
	fdset := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}

	// Add Well-Known Types to FileDescriptorSet if they are referenced but not included
	fdset = e.addWellKnownTypes(fdset)

	// Convert to protoreflect.FileDescriptor
	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return "", fmt.Errorf("failed to create file descriptor: %w", err)
	}

	// Get the first (and only) file
	var result string
	var exportErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		var buf bytes.Buffer
		if err := e.printer.PrintProtoFile(fd, &buf); err != nil {
			// Store error for return
			exportErr = fmt.Errorf("failed to export proto: %w", err)
			return false
		}
		result = buf.String()
		return true
	})

	if exportErr != nil {
		return "", exportErr
	}

	// Fix Editions syntax format if needed
	if fdp.Edition != nil {
		result = fixEditionsSyntax(result, fdp.Edition)
	}

	// Fix proto3 optional fields
	result = fixProto3Optional(result, fdp)

	return result, nil
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

// ConvertToFileDescriptor converts a FileDescriptorProto to protoreflect.FileDescriptor.
// This is a helper function for cases where you need the intermediate representation.
func ConvertToFileDescriptor(fdp *descriptorpb.FileDescriptorProto) (protoreflect.FileDescriptor, error) {
	// Create a FileDescriptorSet with just this file
	fdset := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}

	// Convert to protoreflect.FileDescriptor
	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, fmt.Errorf("failed to create file descriptor: %w", err)
	}

	// Get the first (and only) file
	var result protoreflect.FileDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		result = fd
		return false // stop after first file
	})

	if result == nil {
		return nil, fmt.Errorf("no file descriptor found")
	}

	return result, nil
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

// fixProto3Optional adds the 'optional' keyword to proto3 optional fields.
// This is a workaround for protoprint v2.0.0-beta.2 not properly handling proto3_optional.
func fixProto3Optional(content string, fdp *descriptorpb.FileDescriptorProto) string {
	// Only process proto3 files
	if fdp.GetSyntax() != "proto3" {
		return content
	}

	// Process each message
	for _, msg := range fdp.MessageType {
		content = fixProto3OptionalInMessage(content, msg)
	}

	return content
}

// fixProto3OptionalInMessage processes a single message for proto3 optional fields.
func fixProto3OptionalInMessage(content string, msg *descriptorpb.DescriptorProto) string {
	lines := strings.Split(content, "\n")

	for _, field := range msg.Field {
		if field.GetProto3Optional() {
			// Find the field declaration and add 'optional' keyword
			fieldPattern := fmt.Sprintf("%s %s = %d",
				getFieldTypeName(field),
				field.GetName(),
				field.GetNumber())

			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.Contains(trimmed, fieldPattern) && !strings.HasPrefix(trimmed, "optional ") {
					// Add 'optional' keyword
					indent := line[:len(line)-len(trimmed)]
					lines[i] = indent + "optional " + trimmed
					break
				}
			}
		}
	}

	// Process nested messages
	for _, nested := range msg.NestedType {
		content = strings.Join(lines, "\n")
		content = fixProto3OptionalInMessage(content, nested)
		lines = strings.Split(content, "\n")
	}

	return strings.Join(lines, "\n")
}

// getFieldTypeName returns the type name for a field.
func getFieldTypeName(field *descriptorpb.FieldDescriptorProto) string {
	// Handle message and enum types
	if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
		field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		// Remove leading dots and package prefix for cleaner output
		typeName := field.GetTypeName()
		typeName = strings.TrimPrefix(typeName, ".")
		parts := strings.Split(typeName, ".")
		return parts[len(parts)-1]
	}

	// Handle scalar types
	return getScalarTypeName(field.GetType())
}

// getScalarTypeName returns the type name for scalar types.
func getScalarTypeName(fieldType descriptorpb.FieldDescriptorProto_Type) string {
	// Map of field types to their string representations
	typeNames := map[descriptorpb.FieldDescriptorProto_Type]string{
		descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:   "double",
		descriptorpb.FieldDescriptorProto_TYPE_FLOAT:    "float",
		descriptorpb.FieldDescriptorProto_TYPE_INT64:    "int64",
		descriptorpb.FieldDescriptorProto_TYPE_UINT64:   "uint64",
		descriptorpb.FieldDescriptorProto_TYPE_INT32:    "int32",
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  "fixed64",
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  "fixed32",
		descriptorpb.FieldDescriptorProto_TYPE_BOOL:     "bool",
		descriptorpb.FieldDescriptorProto_TYPE_STRING:   "string",
		descriptorpb.FieldDescriptorProto_TYPE_BYTES:    "bytes",
		descriptorpb.FieldDescriptorProto_TYPE_UINT32:   "uint32",
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: "sfixed32",
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: "sfixed64",
		descriptorpb.FieldDescriptorProto_TYPE_SINT32:   "sint32",
		descriptorpb.FieldDescriptorProto_TYPE_SINT64:   "sint64",
		descriptorpb.FieldDescriptorProto_TYPE_GROUP:    "group", // deprecated but still in the enum
		descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:  "message",
		descriptorpb.FieldDescriptorProto_TYPE_ENUM:     "enum",
	}

	if name, ok := typeNames[fieldType]; ok {
		return name
	}
	return "unknown"
}

// addWellKnownTypes adds missing Well-Known Types to the FileDescriptorSet
func (e *Exporter) addWellKnownTypes(fdset *descriptorpb.FileDescriptorSet) *descriptorpb.FileDescriptorSet {
	// Map of Well-Known Type import paths
	wellKnownImports := map[string]bool{
		"google/protobuf/timestamp.proto":  false,
		"google/protobuf/duration.proto":   false,
		"google/protobuf/empty.proto":      false,
		"google/protobuf/struct.proto":     false,
		"google/protobuf/wrappers.proto":   false,
		"google/protobuf/field_mask.proto": false,
		"google/protobuf/any.proto":        false,
	}

	// Check which Well-Known Types are referenced
	for _, file := range fdset.File {
		for _, dep := range file.Dependency {
			if _, ok := wellKnownImports[dep]; ok {
				wellKnownImports[dep] = true
			}
		}
	}

	// Check if any Well-Known Types are already included
	existingFiles := make(map[string]bool)
	for _, file := range fdset.File {
		if file.Name != nil {
			existingFiles[*file.Name] = true
		}
	}

	// Create a new FileDescriptorSet with Well-Known Types added
	result := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(fdset.File)),
	}

	// Add Well-Known Type descriptors that are referenced but not included
	for importPath, isReferenced := range wellKnownImports {
		if isReferenced && !existingFiles[importPath] {
			// Get the Well-Known Type descriptor from the global registry
			fd, err := protoregistry.GlobalFiles.FindFileByPath(importPath)
			if err == nil {
				fdp := protodesc.ToFileDescriptorProto(fd)
				result.File = append(result.File, fdp)
			}
		}
	}

	// Add all original files
	result.File = append(result.File, fdset.File...)

	return result
}
