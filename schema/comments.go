// Package schema provides comment and documentation support for proto generation.
package schema

import (
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// CommentInfo holds documentation comments for a proto element.
type CommentInfo struct {
	Leading  string   // Comment appearing before the element
	Trailing string   // Comment appearing after the element on same line
	Detached []string // Detached comment blocks appearing before the element
}

// PathBuilder helps build paths for SourceCodeInfo locations.
type PathBuilder struct {
	path []int32
}

// NewPathBuilder creates a new path builder.
func NewPathBuilder() *PathBuilder {
	return &PathBuilder{
		path: make([]int32, 0, 10), // Pre-allocate for typical path depth
	}
}

// Push adds a field number or index to the path.
func (p *PathBuilder) Push(fieldNumberOrIndex int32) *PathBuilder {
	p.path = append(p.path, fieldNumberOrIndex)
	return p
}

// Pop removes the last element from the path.
func (p *PathBuilder) Pop() *PathBuilder {
	if len(p.path) > 0 {
		p.path = p.path[:len(p.path)-1]
	}
	return p
}

// Build returns a copy of the current path.
func (p *PathBuilder) Build() []int32 {
	result := make([]int32, len(p.path))
	copy(result, p.path)
	return result
}

// Reset clears the path.
func (p *PathBuilder) Reset() *PathBuilder {
	p.path = p.path[:0]
	return p
}

// Clone creates a copy of the path builder.
func (p *PathBuilder) Clone() *PathBuilder {
	newPath := make([]int32, len(p.path))
	copy(newPath, p.path)
	return &PathBuilder{path: newPath}
}

// SourceCodeInfoBuilder helps build SourceCodeInfo for a FileDescriptorProto.
type SourceCodeInfoBuilder struct {
	locations []*descriptorpb.SourceCodeInfo_Location
}

// NewSourceCodeInfoBuilder creates a new SourceCodeInfo builder.
func NewSourceCodeInfoBuilder() *SourceCodeInfoBuilder {
	return &SourceCodeInfoBuilder{
		locations: make([]*descriptorpb.SourceCodeInfo_Location, 0),
	}
}

// AddLocation adds a location with comments to the SourceCodeInfo.
func (b *SourceCodeInfoBuilder) AddLocation(path []int32, comment *CommentInfo) {
	if comment == nil || (comment.Leading == "" && comment.Trailing == "" && len(comment.Detached) == 0) {
		return // No comments to add
	}

	// Defensive check: don't add locations with empty paths
	if len(path) == 0 {
		return
	}

	location := &descriptorpb.SourceCodeInfo_Location{
		Path: path,
		// Add dummy span since we're generating from Go structs, not parsing .proto files
		// Span format: [start_line, start_column, end_line, end_column] (all 0-based)
		Span: []int32{0, 0, 0, 0},
	}

	if comment.Leading != "" {
		location.LeadingComments = proto(formatComment(comment.Leading))
	}

	if comment.Trailing != "" {
		location.TrailingComments = proto(formatComment(comment.Trailing))
	}

	if len(comment.Detached) > 0 {
		location.LeadingDetachedComments = make([]string, len(comment.Detached))
		for i, detached := range comment.Detached {
			location.LeadingDetachedComments[i] = formatComment(detached)
		}
	}

	b.locations = append(b.locations, location)
}

// Build creates the SourceCodeInfo from all added locations.
func (b *SourceCodeInfoBuilder) Build() *descriptorpb.SourceCodeInfo {
	if len(b.locations) == 0 {
		return nil
	}

	// Filter out any locations with empty paths (defensive programming)
	validLocations := make([]*descriptorpb.SourceCodeInfo_Location, 0, len(b.locations))
	for _, loc := range b.locations {
		if len(loc.Path) > 0 {
			validLocations = append(validLocations, loc)
		}
	}

	if len(validLocations) == 0 {
		return nil
	}

	return &descriptorpb.SourceCodeInfo{
		Location: validLocations,
	}
}

// formatComment formats a comment string for proto output.
// It ensures proper spacing and handles multi-line comments.
func formatComment(comment string) string {
	if comment == "" {
		return ""
	}

	// Trim leading/trailing whitespace
	comment = strings.TrimSpace(comment)

	// Handle multi-line comments
	lines := strings.Split(comment, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Join with space before each continuation line
	if len(lines) > 1 {
		result := lines[0]
		for i := 1; i < len(lines); i++ {
			if lines[i] != "" {
				result += "\n " + lines[i]
			} else {
				result += "\n"
			}
		}
		return result
	}

	return comment
}

// ExtractCommentFromTag extracts a comment from a struct tag.
// It looks for the "doc" key in the tag.
func ExtractCommentFromTag(tag string) string {
	if tag == "" {
		return ""
	}

	// Simple extraction - can be improved with proper tag parsing
	const docPrefix = `doc:"`
	idx := strings.Index(tag, docPrefix)
	if idx == -1 {
		return ""
	}

	start := idx + len(docPrefix)
	// Find the closing quote, handling escaped quotes
	end := start
	for end < len(tag) {
		if tag[end] == '\\' && end+1 < len(tag) {
			end += 2 // Skip escaped character
			continue
		}
		if tag[end] == '"' {
			break
		}
		end++
	}

	if end >= len(tag) {
		return ""
	}

	return tag[start:end]
}

// ExtractProtoDoc extracts message-level documentation from a special protoDoc tag.
func ExtractProtoDoc(tag string) string {
	if tag == "" {
		return ""
	}

	const protoDocPrefix = `protoDoc:"`
	idx := strings.Index(tag, protoDocPrefix)
	if idx == -1 {
		return ""
	}

	start := idx + len(protoDocPrefix)
	// Find the closing quote, handling escaped quotes
	end := start
	for end < len(tag) {
		if tag[end] == '\\' && end+1 < len(tag) {
			end += 2 // Skip escaped character
			continue
		}
		if tag[end] == '"' {
			break
		}
		end++
	}

	if end >= len(tag) {
		return ""
	}

	return tag[start:end]
}

// Field numbers for various descriptor types (from descriptor.proto)
const (
	// FileDescriptorProto field numbers
	FileDescriptorProtoNameField        = 1
	FileDescriptorProtoPackageField     = 2
	FileDescriptorProtoDependencyField  = 3
	FileDescriptorProtoMessageTypeField = 4
	FileDescriptorProtoEnumTypeField    = 5
	FileDescriptorProtoServiceField     = 6
	FileDescriptorProtoExtensionField   = 7
	FileDescriptorProtoOptionsField     = 8
	FileDescriptorProtoSourceCodeInfo   = 9

	// DescriptorProto (Message) field numbers
	DescriptorProtoNameField       = 1
	DescriptorProtoFieldField      = 2
	DescriptorProtoExtensionField  = 6
	DescriptorProtoNestedTypeField = 3
	DescriptorProtoEnumTypeField   = 4
	DescriptorProtoOneofDeclField  = 8

	// FieldDescriptorProto field numbers
	FieldDescriptorProtoNameField   = 1
	FieldDescriptorProtoNumberField = 3
	FieldDescriptorProtoLabelField  = 4
	FieldDescriptorProtoTypeField   = 5

	// ServiceDescriptorProto field numbers
	ServiceDescriptorProtoNameField   = 1
	ServiceDescriptorProtoMethodField = 2

	// MethodDescriptorProto field numbers
	MethodDescriptorProtoNameField       = 1
	MethodDescriptorProtoInputTypeField  = 2
	MethodDescriptorProtoOutputTypeField = 3
)
