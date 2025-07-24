# Well-Known Types Example

This example demonstrates using Google's Well-Known Types with Hyperway:

- `google.protobuf.Struct` - Dynamic JSON-like structures
- `google.protobuf.Value` - Any JSON value (string, number, bool, null, list, or struct)
- `google.protobuf.ListValue` - List of mixed types
- `google.protobuf.FieldMask` - Partial updates

## Running the Example

```bash
go run main.go
```

## Testing with curl

### 1. Update Config with FieldMask (JSON)

```bash
curl -X POST http://localhost:8080/config.v1.ConfigService/UpdateConfig \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "theme": "dark",
      "language": "ja",
      "notifications": {
        "email": true,
        "push": false
      }
    },
    "update_mask": {
      "paths": ["theme", "notifications.email"]
    }
  }'
```

### 2. Process Flexible Data (JSON)

```bash
curl -X POST http://localhost:8080/config.v1.ConfigService/ProcessFlexibleData \
  -H "Content-Type: application/json" \
  -d '{
    "single_value": "Hello, World!",
    "mixed_list": {
      "values": [
        {"string_value": "text"},
        {"number_value": 42.5},
        {"bool_value": true}
      ]
    },
    "properties": {
      "name": {"string_value": "test"},
      "count": {"number_value": 100},
      "active": {"bool_value": true}
    }
  }'
```

## Export Proto Files

```bash
# Make sure the server is running first
hyperway proto export --endpoint http://localhost:8080 --output ./proto
```

## Use Cases

### 1. Dynamic Configuration (Struct)
Perfect for settings that vary by user or deployment:
```go
settings, _ := structpb.NewStruct(map[string]interface{}{
    "theme": "dark",
    "language": "ja",
    "features": map[string]interface{}{
        "beta": true,
        "experimental": false,
    },
})
```

### 2. Flexible Values (Value)
When field types can vary:
```go
// Can be string, number, bool, etc.
value := structpb.NewStringValue("hello")
value = structpb.NewNumberValue(42.5)
value = structpb.NewBoolValue(true)
```

### 3. Mixed Type Lists (ListValue)
For heterogeneous arrays:
```go
list, _ := structpb.NewList([]interface{}{
    "string",
    42,
    true,
    nil,
})
```

### 4. Partial Updates (FieldMask)
Update only specific fields:
```go
mask := &fieldmaskpb.FieldMask{
    Paths: []string{"user.name", "user.email"},
}
```