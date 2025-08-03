// Package schema provides functionality to convert Go types to Protobuf FileDescriptorSet.
package schema

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	// Import well-known types to register them
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
)

// ErrSkipField is returned when a field should be skipped during processing.
var ErrSkipField = errors.New("skip field")

// Constants
const (
	mapKeyFieldNumber       = 1
	mapValueFieldNumber     = 2
	underscoreOverheadRatio = 10
)

// title capitalizes the first letter of a string
func title(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Builder converts Go types to Protobuf FileDescriptorSet.
type Builder struct {
	mu          sync.RWMutex
	cache       map[reflect.Type]protoreflect.MessageDescriptor
	fileCache   map[string]*descriptorpb.FileDescriptorProto
	packageName string
	options     BuilderOptions

	// Track all messages being built in current operation
	currentFile      *descriptorpb.FileDescriptorProto
	messageTypes     map[string]*descriptorpb.DescriptorProto
	pendingTypes     []pendingType
	wellKnownImports map[string]bool // Track well-known type imports

	// Comment tracking
	sourceCodeInfo  *SourceCodeInfoBuilder
	pathBuilder     *PathBuilder
	messageIndices  map[string]int32               // Track message indices for path building
	messageComments map[string]*CommentInfo        // Store comments until indices are available
	fieldComments   map[string][]*fieldCommentInfo // Store field comments until indices are available
}

type pendingType struct {
	rt   reflect.Type
	name string
}

type fieldCommentInfo struct {
	fieldIndex int32
	comment    *CommentInfo
}

// BuilderOptions configures the schema builder.
type BuilderOptions struct {
	// PackageName is the protobuf package name to use
	PackageName string
	// EnablePGO enables Profile-Guided Optimization
	EnablePGO bool
	// MaxCacheSize limits the cache size (0 = unlimited)
	MaxCacheSize int

	// SyntaxMode specifies proto3 or editions mode
	SyntaxMode SyntaxMode
	// Edition specifies the edition year (e.g., "2023", "2024")
	Edition string
	// Features specifies the default feature set for editions mode
	Features *FeatureSet
}

// Cache size constants for pre-allocation
const (
	defaultMessageCacheSize = 32
	defaultFileCacheSize    = 16
	defaultMessageTypesSize = 32
	oneofFieldRatio         = 4 // Estimate 1 oneof per 4 fields
)

// NewBuilder creates a new schema builder.
func NewBuilder(opts BuilderOptions) *Builder {
	if opts.PackageName == "" {
		opts.PackageName = "hyperway.v1"
	}

	// Set default features based on syntax mode
	if opts.SyntaxMode == SyntaxEditions {
		if opts.Edition == "" {
			opts.Edition = Edition2023 // Default to 2023
		}
		if opts.Features == nil {
			opts.Features = DefaultEdition2023Features()
		}
	} else if opts.Features == nil {
		// Proto3 mode (default)
		opts.Features = DefaultProto3Features()
	}

	return &Builder{
		// Pre-allocate maps with reasonable initial capacities
		cache:       make(map[reflect.Type]protoreflect.MessageDescriptor, defaultMessageCacheSize),
		fileCache:   make(map[string]*descriptorpb.FileDescriptorProto, defaultFileCacheSize),
		packageName: opts.PackageName,
		options:     opts,
	}
}

// BuildMessage converts a Go type to a protoreflect.MessageDescriptor.
// BuildMessage builds a protoreflect.MessageDescriptor from a Go struct type.
func (b *Builder) BuildMessage(rt reflect.Type) (protoreflect.MessageDescriptor, error) {
	// Check cache first
	if md := b.getCachedMessage(rt); md != nil {
		return md, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check after acquiring write lock
	if md, ok := b.cache[rt]; ok {
		return md, nil
	}

	// Validate and prepare the type
	rt, name, err := b.prepareType(rt)
	if err != nil {
		return nil, err
	}

	// Initialize build context
	b.initializeBuildContext(name)

	// Build all message types
	if err := b.buildAllMessageTypes(rt, name); err != nil {
		return nil, err
	}

	// Add comments and imports
	b.addCommentsToFile()
	b.addImportsToFile()

	// Finalize the file
	b.finalizeFile(name)

	// Create and cache the message descriptor
	return b.createAndCacheDescriptor(rt, name)
}

// getCachedMessage returns a cached message descriptor if available.
func (b *Builder) getCachedMessage(rt reflect.Type) protoreflect.MessageDescriptor {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if md, ok := b.cache[rt]; ok {
		return md
	}
	return nil
}

// prepareType validates and prepares the reflect.Type for processing.
func (b *Builder) prepareType(rt reflect.Type) (reflect.Type, string, error) {
	// Ensure we have a struct type
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return nil, "", fmt.Errorf("type %v is not a struct", rt)
	}

	name := rt.Name()
	if name == "" {
		name = "AnonymousMessage"
	}

	return rt, name, nil
}

// initializeBuildContext sets up the builder's context for a new build.
func (b *Builder) initializeBuildContext(name string) {
	// Initialize file descriptor
	b.currentFile = &descriptorpb.FileDescriptorProto{
		Name:    proto(fmt.Sprintf("%s/%s.proto", b.packageName, strings.ToLower(name))),
		Package: proto(b.packageName),
	}

	// Set syntax based on mode
	if b.options.SyntaxMode == SyntaxEditions {
		b.currentFile.Syntax = proto("editions")
		b.currentFile.Edition = StringToEdition(b.options.Edition)

		// Set file-level features for editions mode
		if b.currentFile.Options == nil {
			b.currentFile.Options = &descriptorpb.FileOptions{}
		}
		fileFeatures := CreateFileFeatures(b.options.Edition)
		ApplyFeaturesToFileOptions(b.currentFile.Options, fileFeatures)
	} else {
		b.currentFile.Syntax = proto("proto3")
	}

	// Pre-allocate collections
	b.messageTypes = make(map[string]*descriptorpb.DescriptorProto, defaultMessageTypesSize)
	b.pendingTypes = nil
	b.wellKnownImports = make(map[string]bool)

	// Initialize comment tracking
	b.sourceCodeInfo = NewSourceCodeInfoBuilder()
	b.pathBuilder = NewPathBuilder()
	b.messageIndices = make(map[string]int32)
	b.messageComments = make(map[string]*CommentInfo)
	b.fieldComments = make(map[string][]*fieldCommentInfo)
}

// buildAllMessageTypes builds the main message and all dependent types.
func (b *Builder) buildAllMessageTypes(rt reflect.Type, name string) error {
	// Build the main message
	visited := make(map[reflect.Type]bool)
	msgProto, err := b.collectMessageType(rt, name, visited)
	if err != nil {
		return err
	}
	b.messageTypes[name] = msgProto

	// Process all pending types
	for len(b.pendingTypes) > 0 {
		pending := b.pendingTypes
		b.pendingTypes = nil

		for _, p := range pending {
			if _, exists := b.messageTypes[p.name]; !exists {
				msg, err := b.collectMessageType(p.rt, p.name, visited)
				if err != nil {
					return err
				}
				b.messageTypes[p.name] = msg
			}
		}
	}

	// Add all collected messages to the file
	b.currentFile.MessageType = make([]*descriptorpb.DescriptorProto, 0, len(b.messageTypes))
	messageIndex := int32(0)
	for msgName, msg := range b.messageTypes {
		b.currentFile.MessageType = append(b.currentFile.MessageType, msg)
		b.messageIndices[msgName] = messageIndex
		messageIndex++
	}

	return nil
}

// addCommentsToFile adds all collected comments to the source code info.
func (b *Builder) addCommentsToFile() {
	// Add message comments
	for msgName, comment := range b.messageComments {
		if idx, ok := b.messageIndices[msgName]; ok {
			path := []int32{FileDescriptorProtoMessageTypeField, idx}
			b.sourceCodeInfo.AddLocation(path, comment)
		}
	}

	// Add field comments
	for msgName, fieldComments := range b.fieldComments {
		if msgIdx, ok := b.messageIndices[msgName]; ok {
			for _, fieldComment := range fieldComments {
				path := []int32{
					FileDescriptorProtoMessageTypeField,
					msgIdx,
					DescriptorProtoFieldField,
					fieldComment.fieldIndex,
				}
				b.sourceCodeInfo.AddLocation(path, fieldComment.comment)
			}
		}
	}
}

// addImportsToFile adds well-known type imports to the file.
func (b *Builder) addImportsToFile() {
	if len(b.wellKnownImports) > 0 {
		b.currentFile.Dependency = make([]string, 0, len(b.wellKnownImports))
		for importPath := range b.wellKnownImports {
			b.currentFile.Dependency = append(b.currentFile.Dependency, importPath)
		}
	}
}

// finalizeFile completes the file setup.
func (b *Builder) finalizeFile(name string) {
	// Attach source code info only if we have locations
	sourceCodeInfo := b.sourceCodeInfo.Build()
	if sourceCodeInfo != nil && len(sourceCodeInfo.Location) > 0 {
		b.currentFile.SourceCodeInfo = sourceCodeInfo
	}

	// Cache the file
	b.fileCache[strings.ToLower(name)] = b.currentFile
}

// createAndCacheDescriptor creates the final message descriptor and caches it.
func (b *Builder) createAndCacheDescriptor(rt reflect.Type, name string) (protoreflect.MessageDescriptor, error) {
	// Build the file descriptor with well-known types registry
	registry, err := b.createFileRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry: %w", err)
	}

	file, err := protodesc.NewFile(b.currentFile, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create file descriptor: %w", err)
	}

	// Get the message descriptor
	md := file.Messages().ByName(protoreflect.Name(name))
	if md == nil {
		return nil, fmt.Errorf("message %s not found in file", name)
	}

	// Cache the result
	if b.options.MaxCacheSize == 0 || len(b.cache) < b.options.MaxCacheSize {
		b.cache[rt] = md
	}

	return md, nil
}

// collectMessageType collects a message type and all its dependencies.
func (b *Builder) collectMessageType(rt reflect.Type, name string, visited map[reflect.Type]bool) (*descriptorpb.DescriptorProto, error) {
	if visited[rt] {
		return b.messageTypes[name], nil // Already processed
	}
	visited[rt] = true

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	msgProto := &descriptorpb.DescriptorProto{
		Name: proto(name),
	}

	// Extract message-level documentation if available
	messageComment := b.extractMessageComment(rt)

	// Detect and add oneof groups
	oneofGroups := detectOneofGroups(rt)
	if err := b.addOneofDescriptors(msgProto, oneofGroups); err != nil {
		return nil, err
	}

	// Process struct fields
	if err := b.processStructFields(rt, msgProto, oneofGroups, visited, name); err != nil {
		return nil, err
	}

	// Store message comment for later processing
	if messageComment != nil && messageComment.Leading != "" {
		b.messageComments[name] = messageComment
	}

	return msgProto, nil
}

// addOneofDescriptors adds oneof descriptors to the message
func (b *Builder) addOneofDescriptors(msgProto *descriptorpb.DescriptorProto, oneofGroups []OneofGroup) error {
	for i, group := range oneofGroups {
		if i > math.MaxInt32 {
			return fmt.Errorf("too many oneof groups: %d exceeds int32 range", i)
		}
		oneofProto := &descriptorpb.OneofDescriptorProto{
			Name: proto(group.Name),
		}
		msgProto.OneofDecl = append(msgProto.OneofDecl, oneofProto)
	}
	return nil
}

// processStructFields processes all fields in a struct
func (b *Builder) processStructFields(rt reflect.Type, msgProto *descriptorpb.DescriptorProto, oneofGroups []OneofGroup, visited map[reflect.Type]bool, name string) error {
	fieldNumber := int32(1)
	// Pre-allocate map with expected capacity based on field count
	processedOneofFields := make(map[string]bool, rt.NumField()/oneofFieldRatio)

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		// Check if this field is a tagged oneof struct
		oneofIndex, processed := b.checkOneofField(&field, oneofGroups, processedOneofFields)
		if oneofIndex >= 0 {
			if !processed {
				group := oneofGroups[oneofIndex]
				if err := b.processEmbeddedOneof(&field, &fieldNumber, msgProto, &group, oneofIndex); err != nil {
					return err
				}
				processedOneofFields[field.Name] = true
			}
			continue
		}

		// Regular field processing
		if err := b.processRegularField(&field, &fieldNumber, msgProto, visited, name); err != nil {
			return err
		}
	}

	return nil
}

// checkOneofField checks if a field is part of a oneof group
func (b *Builder) checkOneofField(field *reflect.StructField, oneofGroups []OneofGroup, processedFields map[string]bool) (int32, bool) {
	for idx, group := range oneofGroups {
		if field.Name == title(group.Name) || strings.EqualFold(field.Name, group.Name) {
			// Safe conversion with bounds check
			if idx < 0 || idx > math.MaxInt32 {
				// This should never happen in practice, but handle it gracefully
				return -1, false
			}
			return int32(idx), processedFields[field.Name]
		}
	}
	return -1, false
}

// processRegularField processes a regular (non-oneof) field
func (b *Builder) processRegularField(field *reflect.StructField, fieldNumber *int32, msgProto *descriptorpb.DescriptorProto, visited map[reflect.Type]bool, name string) error {
	fieldProto, nestedTypes, err := b.buildFieldDescriptor(field, *fieldNumber, visited, name)
	if err != nil {
		if errors.Is(err, ErrSkipField) {
			return nil
		}
		return fmt.Errorf("failed to build field %s: %w", field.Name, err)
	}

	if fieldProto != nil {
		// Extract field comment
		fieldComment := b.extractFieldComment(field)

		// Track field index for comments
		fieldLen := len(msgProto.Field)
		if fieldLen > math.MaxInt32 {
			return fmt.Errorf("too many fields: %d exceeds int32 range", fieldLen)
		}
		fieldIndex := int32(fieldLen)
		msgProto.Field = append(msgProto.Field, fieldProto)

		// Store field comment for later processing
		if fieldComment != nil && fieldComment.Leading != "" {
			if b.fieldComments[name] == nil {
				b.fieldComments[name] = make([]*fieldCommentInfo, 0)
			}
			b.fieldComments[name] = append(b.fieldComments[name], &fieldCommentInfo{
				fieldIndex: fieldIndex,
				comment:    fieldComment,
			})
		}

		*fieldNumber++
	}

	// Add any nested types (like map entries) to this message
	msgProto.NestedType = append(msgProto.NestedType, nestedTypes...)
	return nil
}

// extractFieldName extracts the field name from struct field tags
func (b *Builder) extractFieldName(field *reflect.StructField) (string, bool) {
	fieldName := field.Name

	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" && parts[0] != "-" {
			fieldName = parts[0]
		} else if parts[0] == "-" {
			// Skip fields with json:"-" tag
			return "", true
		}
	}

	return toSnakeCase(fieldName), false
}

// analyzeFieldType analyzes the Go type to determine proto field characteristics
func (b *Builder) analyzeFieldType(ft reflect.Type) (fieldType reflect.Type, isRepeated, isMap, isExplicitlyOptional bool) {
	fieldType = ft

	switch ft.Kind() { //nolint:exhaustive // Other types handled as-is in default
	case reflect.Slice:
		if ft.Elem().Kind() != reflect.Uint8 { // Not []byte
			isRepeated = true
			fieldType = ft.Elem()
		}
	case reflect.Map:
		isMap = true
	case reflect.Ptr:
		// Pointer types are explicitly optional in proto3
		fieldType = ft.Elem()
		// Check if it's a pointer to slice (not supported as optional)
		if fieldType.Kind() != reflect.Slice {
			isExplicitlyOptional = true
		}
	default:
		// All other types are handled as-is
	}

	return
}

// buildFieldDescriptor builds a field descriptor from a struct field.
func (b *Builder) buildFieldDescriptor(
	field *reflect.StructField,
	number int32,
	_ map[reflect.Type]bool,
	parentMessageName string,
) (*descriptorpb.FieldDescriptorProto, []*descriptorpb.DescriptorProto, error) {
	// Extract field name
	fieldName, skip := b.extractFieldName(field)
	if skip {
		return nil, nil, ErrSkipField
	}

	fieldProto := &descriptorpb.FieldDescriptorProto{
		Name:   proto(fieldName),
		Number: proto(number),
	}

	// Analyze field type
	ft, isRepeated, isMap, isExplicitlyOptional := b.analyzeFieldType(field.Type)

	// Set field label
	b.setFieldLabel(fieldProto, isRepeated, isMap, isExplicitlyOptional)

	// Handle special field types
	if isMap {
		return b.buildMapField(field, fieldProto, number, parentMessageName)
	}

	if IsEmptyType(ft, field.Tag) {
		return b.buildEmptyField(fieldProto), nil, nil
	}

	// Set regular field type
	if err := b.setFieldType(fieldProto, ft, field.Name); err != nil {
		return nil, nil, err
	}

	// Apply field tags
	b.applyFieldTags(fieldProto, field, isRepeated, isMap)

	return fieldProto, nil, nil
}

// setFieldLabel sets the field label based on field characteristics and syntax mode.
func (b *Builder) setFieldLabel(fieldProto *descriptorpb.FieldDescriptorProto, isRepeated, isMap, isExplicitlyOptional bool) {
	if isRepeated || isMap {
		fieldProto.Label = labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED)
	} else {
		fieldProto.Label = labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL)

		// Handle field presence based on syntax mode
		if b.options.SyntaxMode == SyntaxEditions {
			// In Editions mode, field presence is controlled by features
			// The default for Edition 2023 is EXPLICIT presence
			// For pointer fields, this is already the correct behavior
			// For non-pointer fields, the features system handles the presence
			// No need to set Proto3Optional in editions mode
		} else if isExplicitlyOptional {
			// Proto3 mode: Set proto3_optional for explicitly optional fields (pointer types)
			fieldProto.Proto3Optional = proto(true)
		}
	}
}

// buildEmptyField builds a field descriptor for Empty type.
func (b *Builder) buildEmptyField(fieldProto *descriptorpb.FieldDescriptorProto) *descriptorpb.FieldDescriptorProto {
	b.wellKnownImports[EmptyProto] = true
	fieldProto.Type = typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	fieldProto.TypeName = proto(WellKnownEmpty)
	return fieldProto
}

// setFieldType sets the field type in the field descriptor.
func (b *Builder) setFieldType(fieldProto *descriptorpb.FieldDescriptorProto, ft reflect.Type, fieldName string) error {
	protoType, typeName, err := b.getFieldType(ft, fieldName)
	if err != nil {
		return err
	}

	fieldProto.Type = typePtr(protoType)
	if typeName != "" {
		fieldProto.TypeName = proto(typeName)
	}
	return nil
}

// applyFieldTags applies validation and proto tags to the field descriptor.
func (b *Builder) applyFieldTags(fieldProto *descriptorpb.FieldDescriptorProto, field *reflect.StructField, isRepeated, isMap bool) {
	// Handle validation tags
	if validateTag := field.Tag.Get("validate"); validateTag != "" {
		AddValidationMetadata(fieldProto, validateTag)
	}

	// Extract all tags for field characteristics
	tags := make(map[string]string)
	if protoTag := field.Tag.Get("proto"); protoTag != "" {
		tags["proto"] = protoTag
	}
	if defaultTag := field.Tag.Get("default"); defaultTag != "" {
		tags["default"] = defaultTag
	}

	if b.options.SyntaxMode == SyntaxEditions {
		// In Editions mode, apply field features
		chars := ExtractFieldCharacteristics(tags)

		// Get parent features (file-level features)
		var parentFeatures *descriptorpb.FeatureSet
		if b.currentFile.Options != nil && b.currentFile.Options.Features != nil {
			parentFeatures = b.currentFile.Options.Features
		}

		// Create field-specific features
		fieldFeatures := CreateFieldFeatures(parentFeatures, chars)

		// Apply features to field options if they differ from parent
		if fieldFeatures != nil && !featuresEqual(parentFeatures, fieldFeatures) {
			if fieldProto.Options == nil {
				fieldProto.Options = &descriptorpb.FieldOptions{}
			}
			ApplyFeaturesToFieldOptions(fieldProto.Options, fieldFeatures)
		}

		// Set default value if specified
		if chars.DefaultValue != "" {
			// For Editions, default values are set directly on the field
			fieldProto.DefaultValue = proto(chars.DefaultValue)
		}
	} else if tags["proto"] == protoTagOptional && !isRepeated && !isMap {
		// Proto3 mode: Support proto:"optional" tag
		fieldProto.Proto3Optional = proto(true)
	}
}

// getFieldType returns the protobuf type for a Go type.
func (b *Builder) getFieldType(ft reflect.Type, fieldName string) (descriptorpb.FieldDescriptorProto_Type, string, error) {
	// Handle pointer types
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}

	// Check for well-known types first
	if wkt, ok := IsWellKnownType(ft); ok {
		// Add import if not already added
		b.wellKnownImports[wkt.ImportPath] = true
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, wkt.TypeName, nil
	}

	// Check for time.Duration (which is int64, not struct)
	const timePackage = "time"
	if ft.PkgPath() == timePackage && ft.Name() == "Duration" {
		b.wellKnownImports[DurationProto] = true
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, WellKnownDuration, nil
	}

	// Delegate to helper function to reduce cyclomatic complexity
	return b.getBasicFieldType(ft, fieldName)
}

// getBasicFieldType handles basic Go types
func (b *Builder) getBasicFieldType(ft reflect.Type, fieldName string) (descriptorpb.FieldDescriptorProto_Type, string, error) {
	switch ft.Kind() { //nolint:exhaustive // Unsupported types handled in default case
	case reflect.String:
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, "", nil
	case reflect.Bool:
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", nil
	case reflect.Int32:
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, "", nil
	case reflect.Int, reflect.Int64:
		return descriptorpb.FieldDescriptorProto_TYPE_INT64, "", nil
	case reflect.Uint32:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT32, "", nil
	case reflect.Uint, reflect.Uint64:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT64, "", nil
	case reflect.Float32:
		return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "", nil
	case reflect.Float64:
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "", nil
	case reflect.Slice:
		if ft.Elem().Kind() == reflect.Uint8 {
			return descriptorpb.FieldDescriptorProto_TYPE_BYTES, "", nil
		}
		return 0, "", fmt.Errorf("unsupported slice type: %v", ft)
	case reflect.Struct:
		typeName := ft.Name()
		if typeName == "" {
			typeName = fmt.Sprintf("%s_Message", title(fieldName))
		}

		// Add to pending types to process
		b.pendingTypes = append(b.pendingTypes, pendingType{
			rt:   ft,
			name: typeName,
		})

		fullTypeName := fmt.Sprintf(".%s.%s", b.packageName, typeName)
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, fullTypeName, nil
	default:
		return 0, "", fmt.Errorf("unsupported field type: %v", ft)
	}
}

// buildMapField handles map field types.
func (b *Builder) buildMapField(
	field *reflect.StructField,
	fieldProto *descriptorpb.FieldDescriptorProto,
	_ int32,
	parentMessageName string,
) (*descriptorpb.FieldDescriptorProto, []*descriptorpb.DescriptorProto, error) {
	mapType := field.Type
	keyType := mapType.Key()
	valueType := mapType.Elem()

	// Create map entry message name
	fieldNameTitle := title(field.Name)
	entryName := fmt.Sprintf("%sEntry", fieldNameTitle)

	// Build the map entry message descriptor
	entryMsg := &descriptorpb.DescriptorProto{
		Name:    proto(entryName),
		Options: &descriptorpb.MessageOptions{MapEntry: proto(true)},
		Field:   []*descriptorpb.FieldDescriptorProto{},
	}

	// Add key field
	keyFieldType, _, err := b.getFieldType(keyType, "key")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid map key type %v: %w", keyType, err)
	}
	keyField := &descriptorpb.FieldDescriptorProto{
		Name:   proto("key"),
		Number: proto(int32(mapKeyFieldNumber)),
		Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
		Type:   typePtr(keyFieldType),
	}
	entryMsg.Field = append(entryMsg.Field, keyField)

	// Add value field
	valueFieldType, valueTypeName, err := b.getFieldType(valueType, "value")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid map value type %v: %w", valueType, err)
	}
	valueField := &descriptorpb.FieldDescriptorProto{
		Name:   proto("value"),
		Number: proto(int32(mapValueFieldNumber)),
		Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
		Type:   typePtr(valueFieldType),
	}
	if valueTypeName != "" {
		valueField.TypeName = proto(valueTypeName)
	}
	entryMsg.Field = append(entryMsg.Field, valueField)

	// Reference the map entry type - it will be nested in the parent message
	fieldProto.Type = typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	fieldProto.TypeName = proto(fmt.Sprintf(".%s.%s.%s", b.packageName, parentMessageName, entryName))
	fieldProto.Label = labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED)

	// Return the field and the nested map entry type
	return fieldProto, []*descriptorpb.DescriptorProto{entryMsg}, nil
}

// Helper functions.
func proto[T any](v T) *T {
	return &v
}

// Note: featuresEqual function moved to features_compare.go to reduce complexity

func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}

func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}

// toSnakeCase converts a string to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	// Pre-allocate capacity assuming ~10% overhead for underscores
	const underscoreOverheadRatio = 10
	result.Grow(len(s) + len(s)/underscoreOverheadRatio)

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}

		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// GetFileDescriptorSet returns the complete FileDescriptorSet with all built messages.
func (b *Builder) GetFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	b.mu.RLock()
	defer b.mu.RUnlock()

	fdset := &descriptorpb.FileDescriptorSet{}
	for _, fileProto := range b.fileCache {
		fdset.File = append(fdset.File, fileProto)
	}
	return fdset
}

// GetSyntaxMode returns the syntax mode of the builder
func (b *Builder) GetSyntaxMode() SyntaxMode {
	return b.options.SyntaxMode
}

// GetEdition returns the edition string
func (b *Builder) GetEdition() string {
	return b.options.Edition
}

// HasWellKnownImports returns true if any well-known imports are used
func (b *Builder) HasWellKnownImports() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.wellKnownImports) > 0
}

// GetWellKnownImports returns the map of well-known imports
func (b *Builder) GetWellKnownImports() map[string]bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to avoid external modifications
	imports := make(map[string]bool)
	for k, v := range b.wellKnownImports {
		imports[k] = v
	}
	return imports
}

// processEmbeddedOneof processes fields within an embedded struct that represents a oneof
func (b *Builder) processEmbeddedOneof(
	field *reflect.StructField,
	fieldNumber *int32,
	msgProto *descriptorpb.DescriptorProto,
	group *OneofGroup,
	oneofIndex int32,
) error {
	structType := field.Type
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct type for oneof group %s, got %v", group.Name, structType.Kind())
	}

	// Process each field in the embedded struct
	for i := 0; i < structType.NumField(); i++ {
		subField := structType.Field(i)
		if !subField.IsExported() {
			continue
		}

		// Build field descriptor for this oneof field
		fieldProto, _, err := b.buildFieldDescriptor(&subField, *fieldNumber, nil, "")
		if err != nil {
			if errors.Is(err, ErrSkipField) {
				continue
			}
			return fmt.Errorf("failed to build oneof field %s.%s: %w", field.Name, subField.Name, err)
		}

		if fieldProto != nil {
			// Set the oneof index
			fieldProto.OneofIndex = proto(oneofIndex)

			// Oneof fields should not have proto3_optional set
			// as they already have presence semantics
			// This applies to both proto3 and editions mode
			fieldProto.Proto3Optional = nil

			// Use just the field name for tagged oneofs
			fieldProto.Name = proto(toSnakeCase(subField.Name))

			msgProto.Field = append(msgProto.Field, fieldProto)
			*fieldNumber++
		}
	}

	return nil
}

// createFileRegistry creates a file registry with well-known types
func (b *Builder) createFileRegistry() (protodesc.Resolver, error) {
	// Create a new Files registry containing well-known types
	files := &protoregistry.Files{}

	// Register all well-known types we're using
	for importPath := range b.wellKnownImports {
		// Get the file descriptor from global registry
		fd, err := protoregistry.GlobalFiles.FindFileByPath(importPath)
		if err != nil {
			// If not found in global registry, continue - the protodesc.NewFile will handle it
			continue
		}
		if err := files.RegisterFile(fd); err != nil {
			return nil, fmt.Errorf("failed to register %s: %w", importPath, err)
		}
	}

	return files, nil
}

// extractMessageComment extracts message-level documentation from a struct type.
func (b *Builder) extractMessageComment(rt reflect.Type) *CommentInfo {
	if rt.NumField() == 0 {
		return nil
	}

	// Check first field for protoDoc tag
	firstField := rt.Field(0)
	if firstField.Type == reflect.TypeOf(struct{}{}) && firstField.Name == "_" {
		if doc := ExtractProtoDoc(string(firstField.Tag)); doc != "" {
			return &CommentInfo{Leading: doc}
		}
	}

	return nil
}

// extractFieldComment extracts field-level documentation from a struct field.
func (b *Builder) extractFieldComment(field *reflect.StructField) *CommentInfo {
	if doc := ExtractCommentFromTag(string(field.Tag)); doc != "" {
		return &CommentInfo{Leading: doc}
	}
	return nil
}
