// Package schema provides well-known types support for protobuf.
package schema

import (
	"reflect"
	"time"
)

// Well-known type names
const (
	WellKnownTimestamp = ".google.protobuf.Timestamp"
	WellKnownDuration  = ".google.protobuf.Duration"
	WellKnownEmpty     = ".google.protobuf.Empty"
	WellKnownAny       = ".google.protobuf.Any"
	WellKnownValue     = ".google.protobuf.Value"
	WellKnownStruct    = ".google.protobuf.Struct"
	WellKnownListValue = ".google.protobuf.ListValue"
	WellKnownFieldMask = ".google.protobuf.FieldMask"
)

// Well-known type import paths
const (
	TimestampProto = "google/protobuf/timestamp.proto"
	DurationProto  = "google/protobuf/duration.proto"
	EmptyProto     = "google/protobuf/empty.proto"
	AnyProto       = "google/protobuf/any.proto"
	StructProto    = "google/protobuf/struct.proto"
	WrappersProto  = "google/protobuf/wrappers.proto"
	FieldMaskProto = "google/protobuf/field_mask.proto"
)

// WellKnownType represents information about a well-known type
type WellKnownType struct {
	TypeName   string
	ImportPath string
}

// wellKnownTypes maps Go types to protobuf well-known types
var wellKnownTypes = map[string]WellKnownType{
	"time.Time": {
		TypeName:   WellKnownTimestamp,
		ImportPath: TimestampProto,
	},
	"time.Duration": {
		TypeName:   WellKnownDuration,
		ImportPath: DurationProto,
	},
	// Struct types
	"google.golang.org/protobuf/types/known/structpb.Struct": {
		TypeName:   WellKnownStruct,
		ImportPath: StructProto,
	},
	"google.golang.org/protobuf/types/known/structpb.Value": {
		TypeName:   WellKnownValue,
		ImportPath: StructProto,
	},
	"google.golang.org/protobuf/types/known/structpb.ListValue": {
		TypeName:   WellKnownListValue,
		ImportPath: StructProto,
	},
	// FieldMask
	"google.golang.org/protobuf/types/known/fieldmaskpb.FieldMask": {
		TypeName:   WellKnownFieldMask,
		ImportPath: FieldMaskProto,
	},
	// Any
	"google.golang.org/protobuf/types/known/anypb.Any": {
		TypeName:   WellKnownAny,
		ImportPath: AnyProto,
	},
}

// IsWellKnownType checks if a reflect.Type is a well-known type
func IsWellKnownType(t reflect.Type) (WellKnownType, bool) {
	if t.Kind() != reflect.Struct {
		return WellKnownType{}, false
	}

	// Check for time.Time and time.Duration
	typeKey := t.PkgPath() + "." + t.Name()
	if wkt, ok := wellKnownTypes[typeKey]; ok {
		return wkt, true
	}

	return WellKnownType{}, false
}

// IsEmptyType checks if a type should be treated as google.protobuf.Empty
func IsEmptyType(t reflect.Type, tag reflect.StructTag) bool {
	// Check for explicit proto:"empty" tag
	if protoTag := tag.Get("proto"); protoTag == "empty" {
		return true
	}

	// Check for struct{} type
	if t.Kind() == reflect.Struct && t.NumField() == 0 {
		return true
	}

	return false
}

// GetTimeType returns the time.Time type
func GetTimeType() reflect.Type {
	return reflect.TypeOf(time.Time{})
}

// GetDurationType returns the time.Duration type
func GetDurationType() reflect.Type {
	return reflect.TypeOf(time.Duration(0))
}

const timePackagePath = "time"

// IsTimeType checks if a type is time.Time
func IsTimeType(t reflect.Type) bool {
	return t.PkgPath() == timePackagePath && t.Name() == "Time"
}

// IsDurationType checks if a type is time.Duration
func IsDurationType(t reflect.Type) bool {
	return t.PkgPath() == timePackagePath && t.Name() == "Duration"
}
