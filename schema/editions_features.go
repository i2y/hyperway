// Package schema provides types and functions for Protobuf Editions features support.
package schema

import (
	protoproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Proto tag constants
const (
	protoTagRequired = "required"
	protoTagImplicit = "implicit"
	protoTagExplicit = "explicit"
	protoTagUnpacked = "unpacked"
	protoTagOptional = "optional"
)

// CreateFileFeatures creates a FeatureSet for file-level features based on the edition.
func CreateFileFeatures(edition string) *descriptorpb.FeatureSet {
	features := &descriptorpb.FeatureSet{}

	switch edition {
	case Edition2023:
		// Edition 2023 defaults
		features.FieldPresence = descriptorpb.FeatureSet_EXPLICIT.Enum()
		features.EnumType = descriptorpb.FeatureSet_OPEN.Enum()
		features.RepeatedFieldEncoding = descriptorpb.FeatureSet_PACKED.Enum()
		features.Utf8Validation = descriptorpb.FeatureSet_VERIFY.Enum()
		features.MessageEncoding = descriptorpb.FeatureSet_LENGTH_PREFIXED.Enum()
		features.JsonFormat = descriptorpb.FeatureSet_ALLOW.Enum()
	default:
		// Default to Edition 2023 behavior
		features.FieldPresence = descriptorpb.FeatureSet_EXPLICIT.Enum()
		features.EnumType = descriptorpb.FeatureSet_OPEN.Enum()
		features.RepeatedFieldEncoding = descriptorpb.FeatureSet_PACKED.Enum()
		features.Utf8Validation = descriptorpb.FeatureSet_VERIFY.Enum()
		features.MessageEncoding = descriptorpb.FeatureSet_LENGTH_PREFIXED.Enum()
		features.JsonFormat = descriptorpb.FeatureSet_ALLOW.Enum()
	}

	return features
}

// CreateFieldFeatures creates features for a field based on its characteristics and parent features.
func CreateFieldFeatures(parentFeatures *descriptorpb.FeatureSet, fieldCharacteristics FieldCharacteristics) *descriptorpb.FeatureSet {
	if parentFeatures == nil {
		return nil
	}

	// Clone parent features
	features := protoproto.Clone(parentFeatures).(*descriptorpb.FeatureSet)

	// Override based on field characteristics
	if fieldCharacteristics.ForceImplicitPresence {
		features.FieldPresence = descriptorpb.FeatureSet_IMPLICIT.Enum()
	} else if fieldCharacteristics.ForceExplicitPresence {
		features.FieldPresence = descriptorpb.FeatureSet_EXPLICIT.Enum()
	} else if fieldCharacteristics.IsRequired {
		features.FieldPresence = descriptorpb.FeatureSet_LEGACY_REQUIRED.Enum()
	}

	// Handle repeated field encoding
	if fieldCharacteristics.ForceUnpacked {
		features.RepeatedFieldEncoding = descriptorpb.FeatureSet_EXPANDED.Enum()
	}

	return features
}

// FieldCharacteristics represents field-specific characteristics that affect features.
type FieldCharacteristics struct {
	IsRequired            bool
	ForceImplicitPresence bool
	ForceExplicitPresence bool
	ForceUnpacked         bool
	DefaultValue          string
}

// ApplyFeaturesToFileOptions applies features to FileOptions for editions mode.
func ApplyFeaturesToFileOptions(fileOptions *descriptorpb.FileOptions, features *descriptorpb.FeatureSet) {
	if fileOptions == nil || features == nil {
		return
	}

	// In editions mode, features are set on the file options
	fileOptions.Features = features
}

// ApplyFeaturesToFieldOptions applies features to FieldOptions for editions mode.
func ApplyFeaturesToFieldOptions(fieldOptions *descriptorpb.FieldOptions, features *descriptorpb.FeatureSet) {
	if fieldOptions == nil || features == nil {
		return
	}

	// In editions mode, features can be overridden at field level
	fieldOptions.Features = features
}

// ApplyFeaturesToMessageOptions applies features to MessageOptions for editions mode.
func ApplyFeaturesToMessageOptions(messageOptions *descriptorpb.MessageOptions, features *descriptorpb.FeatureSet) {
	if messageOptions == nil || features == nil {
		return
	}

	// In editions mode, features can be overridden at message level
	messageOptions.Features = features
}

// ShouldUseProto3Optional determines if proto3_optional should be set based on features.
func ShouldUseProto3Optional(syntaxMode SyntaxMode, features *descriptorpb.FeatureSet, isPointer bool) bool {
	if syntaxMode != SyntaxProto3 {
		// proto3_optional is only for proto3 syntax
		return false
	}

	// In proto3, pointer fields should have proto3_optional set
	// unless they have implicit presence forced
	if isPointer && features != nil && features.GetFieldPresence() != descriptorpb.FeatureSet_IMPLICIT {
		return true
	}

	return false
}

// MergeFeatures merges child features with parent features.
// Child features override parent features where specified.
func MergeFeatures(parent, child *descriptorpb.FeatureSet) *descriptorpb.FeatureSet {
	if parent == nil {
		return child
	}
	if child == nil {
		return protoproto.Clone(parent).(*descriptorpb.FeatureSet)
	}

	// Clone parent and override with child values
	merged := protoproto.Clone(parent).(*descriptorpb.FeatureSet)
	protoproto.Merge(merged, child)
	return merged
}

// ExtractFieldCharacteristics extracts field characteristics from struct tags.
func ExtractFieldCharacteristics(tags map[string]string) FieldCharacteristics {
	chars := FieldCharacteristics{}

	// Check proto tag
	if protoTag, ok := tags["proto"]; ok {
		switch protoTag {
		case protoTagRequired:
			chars.IsRequired = true
		case protoTagImplicit:
			chars.ForceImplicitPresence = true
		case protoTagExplicit:
			chars.ForceExplicitPresence = true
		case protoTagUnpacked:
			chars.ForceUnpacked = true
		}
	}

	// Check for default value
	if defaultTag, ok := tags["default"]; ok {
		chars.DefaultValue = defaultTag
	}

	return chars
}
