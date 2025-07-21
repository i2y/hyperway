// Package schema provides types and constants for Protobuf Editions support.
package schema

import (
	"google.golang.org/protobuf/types/descriptorpb"
)

// SyntaxMode represents the protobuf syntax mode.
type SyntaxMode int

const (
	// SyntaxProto3 represents proto3 syntax mode.
	SyntaxProto3 SyntaxMode = iota
	// SyntaxEditions represents editions syntax mode.
	SyntaxEditions
)

// Edition constants for supported editions.
const (
	Edition2023 = "2023"
	Edition2024 = "2024"
)

// FeatureSet represents the feature configuration for Protobuf Editions.
type FeatureSet struct {
	FieldPresence         FieldPresence
	RepeatedFieldEncoding RepeatedFieldEncoding
	EnumType              EnumType
	UTF8Validation        UTF8Validation
}

// FieldPresence represents the field presence feature.
type FieldPresence int

const (
	// FieldPresenceUnknown is the default unset value.
	FieldPresenceUnknown FieldPresence = iota
	// FieldPresenceExplicit means fields have explicit presence tracking (like proto2 optional).
	FieldPresenceExplicit
	// FieldPresenceImplicit means fields have implicit presence (like proto3 default).
	FieldPresenceImplicit
	// FieldPresenceLegacyRequired means fields are required (like proto2 required).
	FieldPresenceLegacyRequired
)

// RepeatedFieldEncoding represents the repeated field encoding feature.
type RepeatedFieldEncoding int

const (
	// RepeatedFieldEncodingUnknown is the default unset value.
	RepeatedFieldEncodingUnknown RepeatedFieldEncoding = iota
	// RepeatedFieldEncodingPacked means repeated fields are packed.
	RepeatedFieldEncodingPacked
	// RepeatedFieldEncodingExpanded means repeated fields are not packed.
	RepeatedFieldEncodingExpanded
)

// EnumType represents the enum type feature.
type EnumType int

const (
	// EnumTypeUnknown is the default unset value.
	EnumTypeUnknown EnumType = iota
	// EnumTypeOpen allows unknown enum values.
	EnumTypeOpen
	// EnumTypeClosed rejects unknown enum values.
	EnumTypeClosed
)

// UTF8Validation represents the UTF-8 validation feature.
type UTF8Validation int

const (
	// UTF8ValidationUnknown is the default unset value.
	UTF8ValidationUnknown UTF8Validation = iota
	// UTF8ValidationVerify verifies UTF-8 validity.
	UTF8ValidationVerify
	// UTF8ValidationNone skips UTF-8 validation.
	UTF8ValidationNone
)

// DefaultProto3Features returns the default feature set for proto3.
func DefaultProto3Features() *FeatureSet {
	return &FeatureSet{
		FieldPresence:         FieldPresenceImplicit,
		RepeatedFieldEncoding: RepeatedFieldEncodingPacked,
		EnumType:              EnumTypeOpen,
		UTF8Validation:        UTF8ValidationVerify,
	}
}

// DefaultEdition2023Features returns the default feature set for Edition 2023.
func DefaultEdition2023Features() *FeatureSet {
	return &FeatureSet{
		FieldPresence:         FieldPresenceExplicit,
		RepeatedFieldEncoding: RepeatedFieldEncodingPacked,
		EnumType:              EnumTypeOpen,
		UTF8Validation:        UTF8ValidationVerify,
	}
}

// Clone creates a copy of the FeatureSet.
func (fs *FeatureSet) Clone() *FeatureSet {
	if fs == nil {
		return nil
	}
	return &FeatureSet{
		FieldPresence:         fs.FieldPresence,
		RepeatedFieldEncoding: fs.RepeatedFieldEncoding,
		EnumType:              fs.EnumType,
		UTF8Validation:        fs.UTF8Validation,
	}
}

// StringToEdition converts a string edition to the protobuf Edition enum.
func StringToEdition(edition string) *descriptorpb.Edition {
	var e descriptorpb.Edition
	switch edition {
	case Edition2023, "EDITION_2023":
		e = descriptorpb.Edition_EDITION_2023
	case Edition2024, "EDITION_2024":
		e = descriptorpb.Edition_EDITION_2024
	default:
		e = descriptorpb.Edition_EDITION_2023 // Default to 2023
	}
	return &e
}
