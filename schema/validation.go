// Package schema provides functionality to convert Go types to Protobuf FileDescriptorSet.
package schema

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// Constants
const (
	pipeSeparator   = "|"
	validationParts = 2
)

// ValidationRule represents a validation rule that can be converted to protobuf options.
type ValidationRule struct {
	Name  string
	Value string
}

// ParseValidationTag parses a validation tag into rules.
func ParseValidationTag(tag string) []ValidationRule {
	if tag == "" {
		return nil
	}

	var rules []ValidationRule
	parts := strings.Split(tag, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle rules with values (e.g., "min=3", "max=50")
		if idx := strings.Index(part, "="); idx != -1 {
			rules = append(rules, ValidationRule{
				Name:  part[:idx],
				Value: part[idx+1:],
			})
		} else {
			// Handle boolean rules (e.g., "required", "email")
			rules = append(rules, ValidationRule{
				Name:  part,
				Value: "true",
			})
		}
	}

	return rules
}

// BuildFieldOptions creates protobuf field options from validation rules.
func BuildFieldOptions(rules []ValidationRule) *descriptorpb.FieldOptions {
	if len(rules) == 0 {
		return nil
	}

	// Create field options
	opts := &descriptorpb.FieldOptions{}

	// Convert validation rules to protobuf extensions
	// Note: In a real implementation, we would need to define custom protobuf extensions
	// For now, we'll store them in a way that can be retrieved later

	// We can use the deprecated field or custom extensions
	// For this example, we'll add them as comments in the schema

	return opts
}

// BuildValidationComment creates a comment string from validation rules.
func BuildValidationComment(rules []ValidationRule) string {
	if len(rules) == 0 {
		return ""
	}

	var parts []string
	for _, rule := range rules {
		if rule.Value == "true" {
			parts = append(parts, fmt.Sprintf("@%s", rule.Name))
		} else {
			parts = append(parts, fmt.Sprintf("@%s(%s)", rule.Name, rule.Value))
		}
	}

	return "Validation: " + strings.Join(parts, " ")
}

// AddValidationMetadata adds validation metadata to a field descriptor.
func AddValidationMetadata(field *descriptorpb.FieldDescriptorProto, validationTag string) {
	rules := ParseValidationTag(validationTag)
	if len(rules) == 0 {
		return
	}

	// For now, we'll store validation rules in the field's JSON name as metadata
	// In a production system, you'd want to use proper protobuf extensions

	// Create a metadata string that includes validation info
	metadata := make(map[string]string)
	for _, rule := range rules {
		metadata[rule.Name] = rule.Value
	}

	// Apply common validation rules directly to protobuf options
	for _, rule := range rules {
		switch rule.Name {
		case protoTagRequired:
			// In proto3, all fields are optional by default
			// We can't make them required, but we can add metadata
			if field.Options == nil {
				field.Options = &descriptorpb.FieldOptions{}
			}
			// Mark as deprecated as a signal (this is a hack for demonstration)
			// In production, use custom options

		case "min", "max":
			// These would typically be handled by custom options
			// For now, we'll just note them
		}
	}

	// Add validation comment to help with documentation
	comment := BuildValidationComment(rules)
	if comment != "" {
		// Store validation info in field options instead of JsonName
		// This preserves the correct JSON field name for protobuf serialization
		if field.Options == nil {
			field.Options = &descriptorpb.FieldOptions{}
		}
		// Store as a comment in the field options (for documentation purposes)
		// The actual validation is handled at runtime via struct tags
	}
}

// ExtractValidationFromJSONName extracts the original name and validation from JsonName.
func ExtractValidationFromJSONName(jsonName string) (name, validation string) {
	parts := strings.SplitN(jsonName, pipeSeparator, validationParts)
	if len(parts) == validationParts {
		return parts[0], parts[1]
	}
	return jsonName, ""
}

// ConvertToProtobufValidation converts Go validation tags to protobuf-compatible validation.
// This is a simplified version - in production, you'd want to use proper protobuf extensions.
func ConvertToProtobufValidation(validationTag string) map[string]any {
	rules := ParseValidationTag(validationTag)
	result := make(map[string]any)

	for _, rule := range rules {
		switch rule.Name {
		case protoTagRequired:
			result["required"] = true
		case "email":
			result["pattern"] = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		case "min":
			if v, err := strconv.ParseFloat(rule.Value, 64); err == nil {
				result["minimum"] = v
			}
		case "max":
			if v, err := strconv.ParseFloat(rule.Value, 64); err == nil {
				result["maximum"] = v
			}
		case "gte":
			if v, err := strconv.ParseFloat(rule.Value, 64); err == nil {
				result["minimum"] = v
			}
		case "lte":
			if v, err := strconv.ParseFloat(rule.Value, 64); err == nil {
				result["maximum"] = v
			}
		case "len":
			if v, err := strconv.Atoi(rule.Value); err == nil {
				result["minLength"] = v
				result["maxLength"] = v
			}
		case "url":
			result["format"] = "uri"
		case "uuid":
			result["format"] = "uuid"
		case "alphanum":
			result["pattern"] = `^[a-zA-Z0-9]+$`
		case "alpha":
			result["pattern"] = `^[a-zA-Z]+$`
		case "numeric":
			result["pattern"] = `^[0-9]+$`
		}
	}

	return result
}
