# Complete Well-Known Types Example

This example demonstrates all supported Google Well-Known Types in Hyperway:

- `google.protobuf.Timestamp` - Date/time representation
- `google.protobuf.Duration` - Time duration
- `google.protobuf.Empty` - Empty message
- `google.protobuf.Any` - Any protobuf message
- `google.protobuf.Struct` - Dynamic JSON-like structures
- `google.protobuf.Value` - Any JSON value
- `google.protobuf.ListValue` - Heterogeneous lists
- `google.protobuf.FieldMask` - Partial updates

## Running the Example

```bash
go run main.go
```

## Testing with curl

### Complete Request Example

```bash
curl -X POST http://localhost:8080/complete.v1.CompleteService/ProcessComplete \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "empty": {},
    "string_tag": null,
    "config": {
      "theme": "dark",
      "features": {
        "auth": true,
        "beta": false
      }
    },
    "value": "simple string",
    "list": ["text", 123, true, null],
    "update_mask": {
      "paths": ["user.name", "user.email"]
    },
    "properties": {
      "version": 1.5,
      "name": "test",
      "active": true
    }
  }'
```

### Create Any Example

```bash
curl -X POST http://localhost:8080/complete.v1.CompleteService/CreateAny \
  -H "Content-Type: application/json" \
  -d '{}'
```

## Well-Known Types Usage

### Empty
```go
// Using struct{}
type Request struct {
    Empty struct{} `json:"empty"`
}

// Using proto:"empty" tag
type Request struct {
    ShouldBeEmpty string `json:"field" proto:"empty"`
}
```

### Any
```go
// Create Any from a message
import "google.golang.org/protobuf/types/known/anypb"
import "google.golang.org/protobuf/types/known/wrapperspb"

str := &wrapperspb.StringValue{Value: "hello"}
anyMsg, _ := anypb.New(str)

// Extract from Any
var extracted wrapperspb.StringValue
if err := anyMsg.UnmarshalTo(&extracted); err == nil {
    fmt.Println(extracted.Value) // "hello"
}
```

### Struct (Dynamic JSON)
```go
// Create from map
config, _ := structpb.NewStruct(map[string]interface{}{
    "theme": "dark",
    "timeout": 30,
    "features": map[string]interface{}{
        "auth": true,
    },
})
```

### Value (Any JSON Value)
```go
// Different value types
strVal := structpb.NewStringValue("text")
numVal := structpb.NewNumberValue(42.5)
boolVal := structpb.NewBoolValue(true)
nullVal := structpb.NewNullValue()
```

### ListValue (Mixed Types)
```go
// Create heterogeneous list
list, _ := structpb.NewList([]interface{}{
    "string",
    123,
    true,
    nil,
    map[string]interface{}{"nested": "object"},
})
```

### FieldMask (Partial Updates)
```go
// Specify update paths
mask := &fieldmaskpb.FieldMask{
    Paths: []string{
        "user.profile.name",
        "user.settings.theme",
    },
}
```

## JSON Representation

### Any in JSON
The `google.protobuf.Any` type uses a special JSON representation:
```json
{
  "@type": "type.googleapis.com/google.protobuf.StringValue",
  "value": "hello"
}
```

Note: Direct JSON serialization of Any can be complex. For production use, consider using the binary protobuf format or specialized Any handling.