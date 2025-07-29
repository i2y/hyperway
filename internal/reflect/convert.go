// Package reflect provides reflection-based conversion utilities.
package reflect

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fieldNameCache caches snake_case to camelCase conversions
var fieldNameCache = sync.Map{}

// fieldMappingCache caches field mappings for struct types to avoid repeated reflection
var fieldMappingCache = sync.Map{} // map[reflect.Type]map[string]fieldMapping

type fieldMapping struct {
	fieldIndex int
	jsonName   string
	protoName  string
}

// ProtoToStruct converts a protobuf message to a Go struct using reflection.
func ProtoToStruct(msg protoreflect.Message, target any) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	return protoToStructDirect(msg, targetValue.Elem())
}

// StructToProto converts a Go struct to a protobuf message using reflection.
func StructToProto(src any, msg protoreflect.Message) error {
	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() == reflect.Ptr {
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() != reflect.Struct {
		return fmt.Errorf("source must be a struct or pointer to struct")
	}

	return structToProtoDirect(srcValue, msg)
}

// protoToStructDirect directly converts proto to struct using reflection
func protoToStructDirect(msg protoreflect.Message, target reflect.Value) error {
	// Iterate over all fields in the proto message
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		// Find the corresponding struct field
		structField, found := findStructField(target, string(fd.Name()))
		if !found {
			return true // Skip unknown fields
		}

		// Set the field value
		if err := setFieldValue(structField, v, fd); err != nil {
			// Log error but continue processing other fields
			return true
		}
		return true
	})

	return nil
}

// structToProtoDirect directly converts struct to proto using reflection
func structToProtoDirect(src reflect.Value, msg protoreflect.Message) error {
	msgDesc := msg.Descriptor()

	// Iterate over struct fields
	for i := 0; i < src.NumField(); i++ {
		field := src.Field(i)
		fieldType := src.Type().Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get field name from json tag or use field name
		fieldName := fieldType.Name
		if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Convert to snake_case for proto field lookup
		protoFieldName := camelToSnake(fieldName)
		fd := msgDesc.Fields().ByName(protoreflect.Name(protoFieldName))
		if fd == nil {
			// Try exact match
			fd = msgDesc.Fields().ByName(protoreflect.Name(fieldName))
			if fd == nil {
				continue // Skip unknown fields
			}
		}

		// Handle well-known types
		if err := setProtoFieldWithWellKnown(msg, fd, field); err != nil {
			// If not a well-known type or error occurred, use regular conversion
			if err := setProtoValue(msg, fd, field); err != nil {
				return fmt.Errorf("failed to set field %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// setFieldValue sets a struct field value from a proto value
func setFieldValue(field reflect.Value, protoValue protoreflect.Value, fd protoreflect.FieldDescriptor) error {
	// Handle repeated fields
	if fd.Cardinality() == protoreflect.Repeated {
		// Check if the field is a slice
		if field.Kind() != reflect.Slice {
			return fmt.Errorf("repeated field %s requires slice type in struct, got %v", fd.Name(), field.Kind())
		}

		// Get the list
		list := protoValue.List()

		// Create a new slice with the appropriate length
		sliceType := field.Type()
		elemType := sliceType.Elem()
		newSlice := reflect.MakeSlice(sliceType, list.Len(), list.Len())

		// Process each element
		for i := 0; i < list.Len(); i++ {
			elem := newSlice.Index(i)
			listValue := list.Get(i)

			switch fd.Kind() { //nolint:exhaustive
			case protoreflect.BoolKind:
				elem.SetBool(listValue.Bool())
			case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
				elem.SetInt(listValue.Int())
			case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
				elem.SetInt(listValue.Int())
			case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
				elem.SetUint(listValue.Uint())
			case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
				elem.SetUint(listValue.Uint())
			case protoreflect.FloatKind:
				elem.SetFloat(float64(listValue.Float()))
			case protoreflect.DoubleKind:
				elem.SetFloat(listValue.Float())
			case protoreflect.StringKind:
				elem.SetString(listValue.String())
			case protoreflect.BytesKind:
				elem.SetBytes(listValue.Bytes())
			case protoreflect.MessageKind:
				// For message types, need to handle pointers
				if elemType.Kind() == reflect.Ptr {
					// Create new pointer element
					newElem := reflect.New(elemType.Elem())
					if err := protoToStructDirect(listValue.Message(), newElem.Elem()); err != nil {
						return fmt.Errorf("failed to convert repeated message element %d: %w", i, err)
					}
					elem.Set(newElem)
				} else if elemType.Kind() == reflect.Struct {
					if err := protoToStructDirect(listValue.Message(), elem); err != nil {
						return fmt.Errorf("failed to convert repeated message element %d: %w", i, err)
					}
				}
			default:
				return fmt.Errorf("unsupported repeated field kind: %v", fd.Kind())
			}
		}

		field.Set(newSlice)
		return nil
	}

	// Handle non-repeated fields
	switch fd.Kind() { //nolint:exhaustive // EnumKind and GroupKind are not needed
	case protoreflect.BoolKind:
		field.SetBool(protoValue.Bool())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		field.SetInt(protoValue.Int())
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		field.SetInt(protoValue.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		field.SetUint(protoValue.Uint())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		field.SetUint(protoValue.Uint())
	case protoreflect.FloatKind:
		field.SetFloat(float64(protoValue.Float()))
	case protoreflect.DoubleKind:
		field.SetFloat(protoValue.Float())
	case protoreflect.StringKind:
		field.SetString(protoValue.String())
	case protoreflect.BytesKind:
		field.SetBytes(protoValue.Bytes())
	case protoreflect.MessageKind:
		// Handle well-known types
		if err := handleWellKnownProtoToStruct(field, protoValue.Message(), fd); err == nil {
			return nil
		}

		// For nested messages, recursively convert
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			return protoToStructDirect(protoValue.Message(), field.Elem())
		} else if field.Kind() == reflect.Struct {
			return protoToStructDirect(protoValue.Message(), field)
		}
	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
	return nil
}

// snakeToCamel converts snake_case to CamelCase with caching
func snakeToCamel(s string) string {
	// Check cache first
	cacheKey := "s2c:" + s
	if cached, ok := fieldNameCache.Load(cacheKey); ok {
		return cached.(string)
	}

	// Convert snake_case to CamelCase
	parts := strings.Split(s, "_")
	for i := range parts {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	camel := strings.Join(parts, "")

	// Cache the result
	fieldNameCache.Store(cacheKey, camel)
	return camel
}

// getFieldMappings returns cached field mappings for a struct type
func getFieldMappings(structType reflect.Type) map[string]fieldMapping {
	if cached, ok := fieldMappingCache.Load(structType); ok {
		return cached.(map[string]fieldMapping)
	}

	mappings := make(map[string]fieldMapping)

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		mapping := fieldMapping{fieldIndex: i}

		// Get field name from json tag or use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
				mapping.jsonName = parts[0]
			}
		}

		// Store proto field name (snake_case)
		protoFieldName := camelToSnake(fieldName)
		mapping.protoName = protoFieldName

		// Map by multiple keys for fast lookup
		mappings[protoFieldName] = mapping // snake_case
		mappings[fieldName] = mapping      // original name
		if mapping.jsonName != "" {
			mappings[mapping.jsonName] = mapping // json tag name
		}

		// Also map CamelCase version of snake_case
		camelName := snakeToCamel(protoFieldName)
		if camelName != fieldName {
			mappings[camelName] = mapping
		}
	}

	fieldMappingCache.Store(structType, mappings)
	return mappings
}

// findStructField finds a struct field by proto field name using cached mappings
func findStructField(target reflect.Value, protoFieldName string) (reflect.Value, bool) {
	mappings := getFieldMappings(target.Type())

	if mapping, ok := mappings[protoFieldName]; ok {
		return target.Field(mapping.fieldIndex), true
	}

	return reflect.Value{}, false
}

// StructToJSON converts a Go struct to JSON bytes.
func StructToJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// JSONToStruct converts JSON bytes to a Go struct.
func JSONToStruct(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// ConvertViaJSON converts between proto and struct using JSON as intermediate format.
// Deprecated: Use ProtoToStruct or StructToProto for better performance.
func ConvertViaJSON(src, dst any) error {
	// Marshal source to JSON
	jsonData, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Unmarshal JSON to destination
	if err := json.Unmarshal(jsonData, dst); err != nil {
		return fmt.Errorf("failed to unmarshal from JSON: %w", err)
	}

	return nil
}

// CreateDynamicMessage creates a new dynamic protobuf message from a descriptor.
func CreateDynamicMessage(md protoreflect.MessageDescriptor) *dynamicpb.Message {
	return dynamicpb.NewMessage(md)
}

// setProtoValue sets a proto field value from a struct value
func setProtoValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor, value reflect.Value) error { //nolint:gocyclo // Many field types need handling
	// Skip invalid values
	if !value.IsValid() {
		return nil
	}

	// Handle nil pointers
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return nil
	}
	// Handle repeated fields
	if fd.Cardinality() == protoreflect.Repeated {
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}

		// Check if the value is a slice or array
		if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
			return fmt.Errorf("repeated field %s requires slice or array, got %v", fd.Name(), value.Kind())
		}

		// Get or create the list
		list := msg.Mutable(fd).List()

		// Add each element
		for i := 0; i < value.Len(); i++ {
			elem := value.Index(i)

			// Dereference element pointer if needed for scalar types
			elemVal := elem
			if elem.Kind() == reflect.Ptr && !elem.IsNil() && fd.Kind() != protoreflect.MessageKind {
				elemVal = elem.Elem()
			}

			switch fd.Kind() { //nolint:exhaustive
			case protoreflect.BoolKind:
				if elemVal.Kind() != reflect.Bool {
					return fmt.Errorf("repeated field %s: expected bool, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfBool(elemVal.Bool()))
			case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				val := toInt64(elemVal)
				if val < math.MinInt32 || val > math.MaxInt32 {
					return fmt.Errorf("repeated field %s: value %d out of int32 range", fd.Name(), val)
				}
				list.Append(protoreflect.ValueOfInt32(int32(val)))
			case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfInt64(toInt64(elemVal)))
			case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				val := toUint64(elemVal)
				if val > math.MaxUint32 {
					return fmt.Errorf("repeated field %s: value %d out of uint32 range", fd.Name(), val)
				}
				list.Append(protoreflect.ValueOfUint32(uint32(val)))
			case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfUint64(toUint64(elemVal)))
			case protoreflect.FloatKind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfFloat32(float32(toFloat64(elemVal))))
			case protoreflect.DoubleKind:
				if !isNumericKind(elemVal.Kind()) {
					return fmt.Errorf("repeated field %s: expected numeric type, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfFloat64(toFloat64(elemVal)))
			case protoreflect.StringKind:
				if elemVal.Kind() != reflect.String {
					return fmt.Errorf("repeated field %s: expected string, got %v", fd.Name(), elemVal.Kind())
				}
				list.Append(protoreflect.ValueOfString(elemVal.String()))
			case protoreflect.BytesKind:
				switch elemVal.Kind() { //nolint:exhaustive // only handling expected types
				case reflect.Slice:
					if elemVal.Type().Elem().Kind() == reflect.Uint8 {
						list.Append(protoreflect.ValueOfBytes(elemVal.Bytes()))
					} else {
						return fmt.Errorf("repeated field %s: expected []byte, got %v", fd.Name(), elemVal.Type())
					}
				case reflect.String:
					list.Append(protoreflect.ValueOfBytes([]byte(elemVal.String())))
				default:
					return fmt.Errorf("repeated field %s: expected []byte or string, got %v", fd.Name(), elemVal.Kind())
				}
			case protoreflect.MessageKind:
				// For repeated messages, create a new message for each element
				nestedMsg := list.NewElement().Message()
				if elem.Kind() == reflect.Ptr {
					if !elem.IsNil() {
						if err := structToProtoDirect(elem.Elem(), nestedMsg); err != nil {
							return fmt.Errorf("failed to convert repeated message element %d: %w", i, err)
						}
					} else {
						continue // Skip nil pointers
					}
				} else if elem.Kind() == reflect.Struct {
					if err := structToProtoDirect(elem, nestedMsg); err != nil {
						return fmt.Errorf("failed to convert repeated message element %d: %w", i, err)
					}
				}
				list.Append(protoreflect.ValueOfMessage(nestedMsg))
			default:
				return fmt.Errorf("unsupported repeated field kind: %v", fd.Kind())
			}
		}
		return nil
	}

	// Handle non-repeated fields
	switch fd.Kind() { //nolint:exhaustive // EnumKind and GroupKind are not needed
	case protoreflect.BoolKind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a bool
		if value.Kind() != reflect.Bool {
			return fmt.Errorf("expected bool for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfBool(value.Bool()))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		// Check for overflow before conversion
		intVal := toInt64(value)
		if intVal > math.MaxInt32 || intVal < math.MinInt32 {
			return fmt.Errorf("int32 overflow: %d", intVal)
		}
		msg.Set(fd, protoreflect.ValueOfInt32(int32(intVal)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfInt64(toInt64(value)))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		// Check for overflow before conversion
		uintVal := toUint64(value)
		if uintVal > math.MaxUint32 {
			return fmt.Errorf("uint32 overflow: %d", uintVal)
		}
		msg.Set(fd, protoreflect.ValueOfUint32(uint32(uintVal)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfUint64(toUint64(value)))
	case protoreflect.FloatKind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfFloat32(float32(toFloat64(value))))
	case protoreflect.DoubleKind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a numeric type
		if !isNumericKind(value.Kind()) {
			return fmt.Errorf("expected numeric type for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfFloat64(toFloat64(value)))
	case protoreflect.StringKind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Ensure we have a string
		if value.Kind() != reflect.String {
			return fmt.Errorf("expected string for field %s, got %v", fd.Name(), value.Kind())
		}
		msg.Set(fd, protoreflect.ValueOfString(value.String()))
	case protoreflect.BytesKind:
		// Dereference pointer if needed
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		// Handle both []byte and string
		switch value.Kind() { //nolint:exhaustive // only handling expected types
		case reflect.Slice:
			if value.Type().Elem().Kind() == reflect.Uint8 {
				msg.Set(fd, protoreflect.ValueOfBytes(value.Bytes()))
			} else {
				return fmt.Errorf("expected []byte for field %s, got %v", fd.Name(), value.Type())
			}
		case reflect.String:
			msg.Set(fd, protoreflect.ValueOfBytes([]byte(value.String())))
		default:
			return fmt.Errorf("expected []byte or string for field %s, got %v", fd.Name(), value.Kind())
		}
	case protoreflect.MessageKind:
		// For nested messages, recursively convert
		// Don't dereference here, handle it in the condition
		nestedMsg := msg.Mutable(fd).Message()
		if value.Kind() == reflect.Ptr {
			if !value.IsNil() {
				return structToProtoDirect(value.Elem(), nestedMsg)
			}
		} else if value.Kind() == reflect.Struct {
			return structToProtoDirect(value, nestedMsg)
		}
	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
	return nil
}

// camelToSnake converts CamelCase to snake_case with caching
func camelToSnake(s string) string {
	// Check cache first
	cacheKey := "c2s:" + s
	if cached, ok := fieldNameCache.Load(cacheKey); ok {
		return cached.(string)
	}

	// Convert CamelCase to snake_case
	var result strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	snake := strings.ToLower(result.String())

	// Cache the result
	fieldNameCache.Store(cacheKey, snake)
	return snake
}

// CopyProtoFields copies fields from one proto message to another.
func CopyProtoFields(src, dst protoreflect.Message) error {
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		dst.Set(fd, v)
		return true
	})
	return nil
}

// IsZeroValue checks if a reflect.Value is the zero value for its type.
func IsZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid, reflect.UnsafePointer:
		// These kinds are not supported and should not be used
		return true
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	case reflect.String:
		return v.String() == ""
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !IsZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Well-known type conversion functions

// TimeToTimestamp converts time.Time to google.protobuf.Timestamp
func TimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

// TimestampToTime converts google.protobuf.Timestamp to time.Time
func TimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// DurationToDurationPB converts time.Duration to google.protobuf.Duration
func DurationToDurationPB(d time.Duration) *durationpb.Duration {
	return durationpb.New(d)
}

// DurationPBToDuration converts google.protobuf.Duration to time.Duration
func DurationPBToDuration(d *durationpb.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return d.AsDuration()
}

// Helper functions for numeric conversions

// isNumericKind checks if a reflect.Kind is numeric
func isNumericKind(k reflect.Kind) bool {
	switch k { //nolint:exhaustive // only checking numeric types
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// toInt64 converts any numeric value to int64
func toInt64(v reflect.Value) int64 {
	switch v.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()) //nolint:gosec // conversion between numeric types in protobuf context
	case reflect.Float32, reflect.Float64:
		return int64(v.Float())
	default:
		return 0
	}
}

// toUint64 converts any numeric value to uint64
func toUint64(v reflect.Value) uint64 {
	switch v.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ival := v.Int()
		if ival < 0 {
			return 0 // Negative values are clamped to 0 for unsigned types
		}
		return uint64(ival)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	default:
		return 0
	}
}

// toFloat64 converts any numeric value to float64
func toFloat64(v reflect.Value) float64 {
	switch v.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		return 0
	}
}

// handleWellKnownProtoToStruct handles conversion from well-known proto types to Go types
func handleWellKnownProtoToStruct(field reflect.Value, msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	typeName := string(fd.Message().FullName())

	switch typeName {
	case "google.protobuf.Timestamp":
		return handleTimestampProtoToStruct(field, msg)
	case "google.protobuf.Duration":
		return handleDurationProtoToStruct(field, msg)
	case "google.protobuf.Empty":
		// Empty message - nothing to do
		return nil
	case "google.protobuf.Struct":
		return handleStructProtoToStruct(field, msg)
	case "google.protobuf.Value":
		return handleValueProtoToStruct(field, msg)
	case "google.protobuf.ListValue":
		return handleListValueProtoToStruct(field, msg)
	case "google.protobuf.FieldMask":
		return handleFieldMaskProtoToStruct(field, msg)
	case "google.protobuf.Any":
		return handleAnyProtoToStruct(field, msg)
	}

	return fmt.Errorf("not a well-known type or unsupported conversion")
}

// handleTimestampProtoToStruct converts Timestamp message to time.Time
func handleTimestampProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(time.Time{}) {
		seconds := msg.Get(msg.Descriptor().Fields().ByName("seconds")).Int()
		nanos := msg.Get(msg.Descriptor().Fields().ByName("nanos")).Int()
		t := time.Unix(seconds, nanos).UTC()
		field.Set(reflect.ValueOf(t))
		return nil
	}
	return fmt.Errorf("field type mismatch for Timestamp")
}

// handleDurationProtoToStruct converts Duration message to time.Duration
func handleDurationProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		seconds := msg.Get(msg.Descriptor().Fields().ByName("seconds")).Int()
		nanos := msg.Get(msg.Descriptor().Fields().ByName("nanos")).Int()
		d := time.Duration(seconds)*time.Second + time.Duration(nanos)*time.Nanosecond
		field.Set(reflect.ValueOf(d))
		return nil
	}
	return fmt.Errorf("field type mismatch for Duration")
}

// handleStructProtoToStruct converts Struct message to *structpb.Struct
func handleStructProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(&structpb.Struct{}) {
		structVal := &structpb.Struct{
			Fields: make(map[string]*structpb.Value),
		}

		// Get the fields map
		fieldsDesc := msg.Descriptor().Fields().ByName("fields")
		if fieldsDesc != nil {
			fieldsMap := msg.Get(fieldsDesc).Map()
			fieldsMap.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
				key := k.String()
				// Convert the value message to structpb.Value
				if valueMsg := v.Message(); valueMsg != nil {
					if value, err := convertToStructpbValue(valueMsg); err == nil {
						structVal.Fields[key] = value
					}
				}
				return true
			})
		}

		field.Set(reflect.ValueOf(structVal))
		return nil
	}
	return fmt.Errorf("field type mismatch for Struct")
}

// handleValueProtoToStruct converts Value message to *structpb.Value
func handleValueProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(&structpb.Value{}) {
		value, err := convertToStructpbValue(msg)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(value))
		return nil
	}
	return fmt.Errorf("field type mismatch for Value")
}

// handleListValueProtoToStruct converts ListValue message to *structpb.ListValue
func handleListValueProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(&structpb.ListValue{}) {
		listVal := &structpb.ListValue{
			Values: make([]*structpb.Value, 0),
		}

		// Get the values repeated field
		valuesDesc := msg.Descriptor().Fields().ByName("values")
		if valuesDesc != nil {
			valuesList := msg.Get(valuesDesc).List()
			for i := 0; i < valuesList.Len(); i++ {
				if valueMsg := valuesList.Get(i).Message(); valueMsg != nil {
					if value, err := convertToStructpbValue(valueMsg); err == nil {
						listVal.Values = append(listVal.Values, value)
					}
				}
			}
		}

		field.Set(reflect.ValueOf(listVal))
		return nil
	}
	return fmt.Errorf("field type mismatch for ListValue")
}

// handleFieldMaskProtoToStruct converts FieldMask message to *fieldmaskpb.FieldMask
func handleFieldMaskProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(&fieldmaskpb.FieldMask{}) {
		fieldMask := &fieldmaskpb.FieldMask{
			Paths: make([]string, 0),
		}

		// Get the paths repeated field
		pathsDesc := msg.Descriptor().Fields().ByName("paths")
		if pathsDesc != nil {
			pathsList := msg.Get(pathsDesc).List()
			for i := 0; i < pathsList.Len(); i++ {
				fieldMask.Paths = append(fieldMask.Paths, pathsList.Get(i).String())
			}
		}

		field.Set(reflect.ValueOf(fieldMask))
		return nil
	}
	return fmt.Errorf("field type mismatch for FieldMask")
}

// handleAnyProtoToStruct converts Any message to *anypb.Any
func handleAnyProtoToStruct(field reflect.Value, msg protoreflect.Message) error {
	if field.Type() == reflect.TypeOf(&anypb.Any{}) {
		anyVal := &anypb.Any{}

		// Get type_url field
		if typeURLDesc := msg.Descriptor().Fields().ByName("type_url"); typeURLDesc != nil {
			anyVal.TypeUrl = msg.Get(typeURLDesc).String()
		}

		// Get value field
		if valueDesc := msg.Descriptor().Fields().ByName("value"); valueDesc != nil {
			anyVal.Value = msg.Get(valueDesc).Bytes()
		}

		field.Set(reflect.ValueOf(anyVal))
		return nil
	}
	return fmt.Errorf("field type mismatch for Any")
}

// convertToStructpbValue converts a protoreflect.Message to a structpb.Value
func convertToStructpbValue(msg protoreflect.Message) (*structpb.Value, error) {
	desc := msg.Descriptor()

	// Handle the oneof field "kind"
	kindField := desc.Oneofs().ByName("kind")
	if kindField == nil {
		return nil, fmt.Errorf("message is not a google.protobuf.Value")
	}

	// Find which field is set in the oneof
	var setField protoreflect.FieldDescriptor
	for i := 0; i < kindField.Fields().Len(); i++ {
		fd := kindField.Fields().Get(i)
		if msg.Has(fd) {
			setField = fd
			break
		}
	}

	if setField == nil {
		// No field is set, return null value
		return structpb.NewNullValue(), nil
	}

	switch setField.Name() {
	case "null_value":
		return structpb.NewNullValue(), nil
	case "number_value":
		return structpb.NewNumberValue(msg.Get(setField).Float()), nil
	case "string_value":
		return structpb.NewStringValue(msg.Get(setField).String()), nil
	case "bool_value":
		return structpb.NewBoolValue(msg.Get(setField).Bool()), nil
	case "struct_value":
		return convertStructValue(msg.Get(setField).Message())
	case "list_value":
		return convertListValue(msg.Get(setField).Message())
	default:
		return nil, fmt.Errorf("unknown Value kind: %s", setField.Name())
	}
}

// convertStructValue converts a protobuf Struct message to structpb.Value
func convertStructValue(structMsg protoreflect.Message) (*structpb.Value, error) {
	structVal := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}

	fieldsDesc := structMsg.Descriptor().Fields().ByName("fields")
	if fieldsDesc != nil {
		fieldsMap := structMsg.Get(fieldsDesc).Map()
		fieldsMap.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			key := k.String()
			if valueMsg := v.Message(); valueMsg != nil {
				if value, err := convertToStructpbValue(valueMsg); err == nil {
					structVal.Fields[key] = value
				}
			}
			return true
		})
	}
	return structpb.NewStructValue(structVal), nil
}

// convertListValue converts a protobuf ListValue message to structpb.Value
func convertListValue(listMsg protoreflect.Message) (*structpb.Value, error) {
	values := make([]*structpb.Value, 0)

	valuesDesc := listMsg.Descriptor().Fields().ByName("values")
	if valuesDesc != nil {
		valuesList := listMsg.Get(valuesDesc).List()
		for i := 0; i < valuesList.Len(); i++ {
			if valueMsg := valuesList.Get(i).Message(); valueMsg != nil {
				if value, err := convertToStructpbValue(valueMsg); err == nil {
					values = append(values, value)
				}
			}
		}
	}
	return structpb.NewListValue(&structpb.ListValue{Values: values}), nil
}

// setProtoFieldWithWellKnown sets a proto field value handling well-known type conversions
func setProtoFieldWithWellKnown(msg protoreflect.Message, fd protoreflect.FieldDescriptor, value reflect.Value) error {
	if fd.Kind() != protoreflect.MessageKind {
		return fmt.Errorf("not a message field")
	}

	typeName := string(fd.Message().FullName())

	switch typeName {
	case "google.protobuf.Timestamp":
		if value.Type() == reflect.TypeOf(time.Time{}) {
			t := value.Interface().(time.Time)
			// Create a Timestamp message
			timestampMsg := msg.Mutable(fd).Message()
			timestampMsg.Set(timestampMsg.Descriptor().Fields().ByName("seconds"), protoreflect.ValueOfInt64(t.Unix()))
			nanos := t.Nanosecond()
			if nanos < 0 || nanos > 999999999 {
				return fmt.Errorf("nanoseconds out of range: %d", nanos)
			}
			timestampMsg.Set(timestampMsg.Descriptor().Fields().ByName("nanos"), protoreflect.ValueOfInt32(int32(nanos))) // #nosec G115 -- bounds already checked
			return nil
		}
	case "google.protobuf.Duration":
		if value.Type() == reflect.TypeOf(time.Duration(0)) {
			d := value.Interface().(time.Duration)
			// Create a Duration message
			durationMsg := msg.Mutable(fd).Message()
			seconds := int64(d / time.Second)
			nanosRemainder := d % time.Second
			if nanosRemainder < 0 || nanosRemainder > 999999999 {
				return fmt.Errorf("nanoseconds out of range: %d", nanosRemainder)
			}
			nanos := int32(nanosRemainder) // #nosec G115 -- bounds already checked
			durationMsg.Set(durationMsg.Descriptor().Fields().ByName("seconds"), protoreflect.ValueOfInt64(seconds))
			durationMsg.Set(durationMsg.Descriptor().Fields().ByName("nanos"), protoreflect.ValueOfInt32(nanos))
			return nil
		}
	case "google.protobuf.Empty":
		// Empty message - create empty message
		msg.Mutable(fd).Message()
		return nil
	case "google.protobuf.Any":
		// Handle *anypb.Any
		if value.Type() == reflect.TypeOf(&anypb.Any{}) {
			if !value.IsNil() {
				anyVal := value.Interface().(*anypb.Any)
				anyMsg := msg.Mutable(fd).Message()
				anyMsg.Set(anyMsg.Descriptor().Fields().ByName("type_url"), protoreflect.ValueOfString(anyVal.TypeUrl))
				anyMsg.Set(anyMsg.Descriptor().Fields().ByName("value"), protoreflect.ValueOfBytes(anyVal.Value))
			}
			return nil
		}
	}

	return fmt.Errorf("not a well-known type or unsupported conversion")
}
