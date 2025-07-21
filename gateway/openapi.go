package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// OpenAPISpec represents an OpenAPI 3.0 specification.
type OpenAPISpec struct {
	OpenAPI    string            `json:"openapi"`
	Info       OpenAPIInfo       `json:"info"`
	Servers    []OpenAPIServer   `json:"servers,omitempty"`
	Paths      map[string]any    `json:"paths"`
	Components OpenAPIComponents `json:"components"`
}

// OpenAPIInfo represents API information.
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// OpenAPIServer represents a server.
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPIComponents holds reusable components.
type OpenAPIComponents struct {
	Schemas map[string]any `json:"schemas"`
}

// GenerateOpenAPI generates an OpenAPI spec from a FileDescriptorSet.
func GenerateOpenAPI(fdset *descriptorpb.FileDescriptorSet, info OpenAPIInfo) (*OpenAPISpec, error) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info:    info,
		Paths:   make(map[string]any),
		Components: OpenAPIComponents{
			Schemas: make(map[string]any),
		},
	}

	// Process each file in the descriptor set
	for _, file := range fdset.File {
		if err := processFile(spec, file); err != nil {
			return nil, fmt.Errorf("failed to process file %s: %w", file.GetName(), err)
		}
	}

	return spec, nil
}

// processFile processes a single file descriptor.
func processFile(spec *OpenAPISpec, file *descriptorpb.FileDescriptorProto) error {
	// Process messages as schemas
	for _, msg := range file.MessageType {
		schema := generateMessageSchema(msg)
		schemaName := fmt.Sprintf("%s.%s", file.GetPackage(), msg.GetName())
		spec.Components.Schemas[schemaName] = schema
	}

	// Process services as paths
	for _, svc := range file.Service {
		if err := processService(spec, file, svc); err != nil {
			return err
		}
	}

	return nil
}

// generateMessageSchema generates a JSON schema for a message.
func generateMessageSchema(msg *descriptorpb.DescriptorProto) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": make(map[string]any),
	}

	properties := schema["properties"].(map[string]any)
	required := []string{}

	for _, field := range msg.Field {
		fieldSchema := generateFieldSchema(field)
		fieldName := field.GetName()
		properties[fieldName] = fieldSchema

		// Check if field is required (not optional in proto3)
		if field.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
			required = append(required, fieldName)
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// generateFieldSchema generates a JSON schema for a field.
func generateFieldSchema(field *descriptorpb.FieldDescriptorProto) map[string]any {
	schema := make(map[string]any)

	// Handle repeated fields
	if field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		schema["type"] = "array"
		schema["items"] = getFieldTypeSchema(field)
		return schema
	}

	// Handle non-repeated fields
	return getFieldTypeSchema(field)
}

// getFieldTypeSchema returns the schema for a field type.
func getFieldTypeSchema(field *descriptorpb.FieldDescriptorProto) map[string]any {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return map[string]any{"type": "string"}
	case descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return map[string]any{"type": "integer"}
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
		descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return map[string]any{"type": "number"}
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return map[string]any{"type": "boolean"}
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return map[string]any{
			"type":   "string",
			"format": "byte",
		}
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		// Reference to another message
		typeName := field.GetTypeName()
		// Remove leading dot from type name
		typeName = strings.TrimPrefix(typeName, ".")

		// Handle well-known types
		if typeName == "google.protobuf.Timestamp" {
			return map[string]any{
				"type":   "string",
				"format": "date-time",
			}
		} else if typeName == "google.protobuf.Duration" {
			return map[string]any{
				"type":   "string",
				"format": "duration",
			}
		} else if typeName == "google.protobuf.Empty" {
			return map[string]any{
				"type": "object",
			}
		}

		return map[string]any{
			"$ref": fmt.Sprintf("#/components/schemas/%s", typeName),
		}
	case descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		// Groups are deprecated, treat as message
		return map[string]any{
			"$ref": fmt.Sprintf("#/components/schemas/%s", field.GetTypeName()),
		}
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		// Enum types
		return map[string]any{
			"type": "string",
			"enum": []string{}, // Would need enum values from descriptor
		}
	default:
		return map[string]any{"type": "string"}
	}
}

// processService processes a service into API paths.
func processService(spec *OpenAPISpec, file *descriptorpb.FileDescriptorProto, svc *descriptorpb.ServiceDescriptorProto) error {
	for _, method := range svc.Method {
		path := fmt.Sprintf("/%s.%s/%s", file.GetPackage(), svc.GetName(), method.GetName())

		// Get input and output types, removing leading dots
		inputType := method.GetInputType()
		outputType := method.GetOutputType()
		inputType = strings.TrimPrefix(inputType, ".")
		outputType = strings.TrimPrefix(outputType, ".")

		operation := map[string]any{
			"operationId": fmt.Sprintf("%s_%s", svc.GetName(), method.GetName()),
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"$ref": fmt.Sprintf("#/components/schemas/%s", inputType),
						},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Success",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": fmt.Sprintf("#/components/schemas/%s", outputType),
							},
						},
					},
				},
			},
		}

		spec.Paths[path] = map[string]any{
			"post": operation,
		}
	}

	return nil
}

// MarshalOpenAPI marshals the OpenAPI spec to JSON.
func MarshalOpenAPI(spec *OpenAPISpec) ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}
