package schema

import "google.golang.org/protobuf/types/descriptorpb"

// featuresEqual compares two FeatureSets for equality.
func featuresEqual(a, b *descriptorpb.FeatureSet) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use helper functions to reduce complexity
	return compareFieldPresence(a.FieldPresence, b.FieldPresence) &&
		compareEnumType(a.EnumType, b.EnumType) &&
		compareRepeatedFieldEncoding(a.RepeatedFieldEncoding, b.RepeatedFieldEncoding) &&
		compareUTF8Validation(a.Utf8Validation, b.Utf8Validation)
}

func compareFieldPresence(a, b *descriptorpb.FeatureSet_FieldPresence) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func compareEnumType(a, b *descriptorpb.FeatureSet_EnumType) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func compareRepeatedFieldEncoding(a, b *descriptorpb.FeatureSet_RepeatedFieldEncoding) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func compareUTF8Validation(a, b *descriptorpb.FeatureSet_Utf8Validation) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}
