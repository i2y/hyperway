# Proto3 vs Editions: Tags and Behavior

This document explains when tags are needed in Proto3 and Editions modes, and how they affect runtime behavior.

## Proto3 Mode

### Default Behavior (No Tags Required)

```go
// Proto3 mode - Most cases don't require tags
type Message struct {
    Name    string   `json:"name"`     // implicit presence (default)
    Count   int32    `json:"count"`    // implicit presence (default)
    Tags    []string `json:"tags"`     // repeated field
}
```

### When Tags Are Needed

```go
// Special cases requiring tags
type Message struct {
    // Pointer types automatically get proto3_optional
    Optional *string `json:"optional"` 
    
    // Non-pointer with explicit presence
    Explicit string  `json:"explicit" proto:"optional"` 
}
```

## Editions Mode (Edition 2023)

### Default Behavior (No Tags Required)

```go
// Edition 2023 - Defaults to explicit presence
type Message struct {
    Name  string  `json:"name"`   // explicit presence (default)
    Count int32   `json:"count"`  // explicit presence (default)
    Tags  []int32 `json:"tags"`   // packed encoding (default)
}
```

### When Tags Are Needed

```go
type Message struct {
    // For proto3-like implicit presence
    Count int32 `json:"count" proto:"implicit"`
    
    // For required fields
    ID string `json:"id" proto:"required"`
    
    // For unpacked repeated fields
    Tags []int32 `json:"tags" proto:"unpacked"`
    
    // For default values
    Name string `json:"name" default:"Unknown"`
}
```

## Runtime Behavior Impact

### 1. Field Presence (Field Tracking)

**Proto3 Example:**
```go
type User struct {
    Name  string `json:"name"`                   // implicit: empty string not sent
    Email string `json:"email" proto:"optional"` // explicit: empty string is sent
}

// Runtime behavior
user := User{Name: "", Email: ""}
// When serialized:
// - Name: not transmitted (implicit presence)
// - Email: transmitted as empty string (explicit presence)
```

**Editions Example:**
```go
type User struct {
    Name  string `json:"name"`                  // explicit: empty string is sent (default)
    Count int32  `json:"count" proto:"implicit"` // implicit: zero not sent
}
```

### 2. Default Values

```go
// Editions only - proto3 doesn't support default values
type Config struct {
    Port    int32  `json:"port" default:"8080"`
    Timeout int32  `json:"timeout" proto:"implicit"` // Cannot have default with implicit
}

// Runtime behavior
config := Config{} // Port: 0, Timeout: 0
// When deserializing:
// - Port: becomes 8080 if field is absent
// - Timeout: remains 0 if field is absent
```

### 3. Serialization Impact

| Presence Type | Zero Value Behavior | Wire Size | Can Distinguish Zero vs Unset |
|--------------|-------------------|-----------|------------------------------|
| Implicit | Not sent | Smaller | No |
| Explicit | Sent | Larger | Yes |
| Required | Must be sent | N/A | Yes (error if missing) |

### 4. Validation

- **Required fields**: Cause error if missing during serialization
- **Other fields**: No error if missing

### 5. Interoperability

- Proto3 and Editions use the same wire format
- Messages can be exchanged between Proto3 and Editions services
- Be aware of presence semantic differences

## Summary

### Proto3 Mode
- **No tags needed in most cases**: Defaults to implicit presence
- **Pointer types**: Automatically get proto3_optional (explicit presence)
- **`proto:"optional"` tag**: Only needed for non-pointer types with explicit presence

### Editions Mode
- **No tags needed in most cases**: Defaults to explicit presence
- **`proto:"implicit"`**: For proto3-like behavior
- **`proto:"required"`**: For mandatory fields
- **`default:"value"`**: For default values
- **`proto:"unpacked"`**: For non-packed repeated fields

### Key Principle

Tags are only needed when you want behavior different from the mode's defaults. They directly affect serialization/deserialization behavior, not just proto file generation.