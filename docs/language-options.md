# Language-Specific Options for Proto Export

This document explains how to use language-specific options when exporting proto files from Hyperway services. These options enable seamless integration with code generation tools for various programming languages.

## Overview

When exporting proto files from Hyperway services, you can now specify language-specific options that will be automatically inserted into the generated proto files. This eliminates the need for manual editing before code generation.

## Supported Language Options

### Go Options
- `go_package` - Specifies the Go package path and optional alias

### Java Options
- `java_package` - The Java package for generated classes
- `java_outer_classname` - Name of the outer class (if not using multiple files)
- `java_multiple_files` - Generate separate file per top-level message

### C# Options
- `csharp_namespace` - The C# namespace for generated code

### PHP Options
- `php_namespace` - PHP namespace for generated classes
- `php_metadata_namespace` - PHP namespace for metadata

### Ruby Options
- `ruby_package` - Ruby package for generated code

### Python Options
- `py_package` - Python package (rarely needed, inferred from proto package)

### Objective-C/Swift Options
- `objc_class_prefix` - Prefix for Objective-C/Swift classes

## Usage Examples

### 1. Using the RPC Package API

#### Basic Usage with Go Package

```go
import (
    "github.com/i2y/hyperway/proto"
    "github.com/i2y/hyperway/rpc"
)

// Create and configure service
svc := rpc.NewService("UserService",
    rpc.WithPackage("user.v1"),
)

// Register methods...

// Export with Go package option
protoContent, err := svc.ExportProtoWithOptions(
    proto.WithGoPackage("github.com/example/api/gen;userv1"),
)
```

#### Multiple Language Options

```go
// Export all protos with multiple language options
files, err := svc.ExportAllProtosWithOptions(
    proto.WithGoPackage("github.com/example/api/gen;userv1"),
    proto.WithJavaPackage("com.example.api.user"),
    proto.WithJavaOuterClass("UserProtos"),
    proto.WithJavaMultipleFiles(true),
    proto.WithCSharpNamespace("Example.Api.User"),
)

// Write files to disk
for filename, content := range files {
    os.WriteFile(filename, []byte(content), 0644)
}
```

### 2. Using the CLI

#### Export with Go Package

```bash
hyperway proto export \
  --endpoint http://localhost:8080 \
  --go-package "github.com/example/api;apiv1"
```

#### Export with Java Options

```bash
hyperway proto export \
  --endpoint http://localhost:8080 \
  --java-package "com.example.api" \
  --java-outer-classname "ApiProtos" \
  --java-multiple-files
```

#### Export with Multiple Languages

```bash
hyperway proto export \
  --endpoint http://localhost:8080 \
  --go-package "github.com/example/api;apiv1" \
  --java-package "com.example.api" \
  --csharp-namespace "Example.Api" \
  --php-namespace "Example\\Api" \
  --ruby-package "Example::Api"
```

### 3. Direct Proto Package API

```go
import "github.com/i2y/hyperway/proto"

// Create export options with language settings
opts := proto.DefaultExportOptions()
opts.ApplyOptions(
    proto.WithGoPackage("github.com/example/api;apiv1"),
    proto.WithJavaPackage("com.example.api"),
    proto.WithCSharpNamespace("Example.Api"),
)

// Create exporter with options
exporter := proto.NewExporter(opts)

// Export FileDescriptorSet
files, err := exporter.ExportFileDescriptorSet(fdset)
```

## Integration with Code Generation Tools

### Connect-go (Go)

```bash
# Export with Go package
go run server/main.go export-proto  # Uses WithGoPackage internally

# Generate Connect-go code
buf generate
```

The exported proto will contain:
```proto
option go_package = "github.com/example/api/gen;userv1";
```

### gRPC Java

```bash
# Export with Java options
hyperway proto export \
  --endpoint http://localhost:8080 \
  --java-package "com.example.api" \
  --java-multiple-files

# Generate Java code
protoc --java_out=./java/src \
       --grpc-java_out=./java/src \
       user.v1.proto
```

The exported proto will contain:
```proto
option java_package = "com.example.api";
option java_multiple_files = true;
```

### gRPC C#

```bash
# Export with C# namespace
hyperway proto export \
  --endpoint http://localhost:8080 \
  --csharp-namespace "Example.Api"

# Generate C# code
protoc --csharp_out=./csharp \
       --grpc_out=./csharp \
       user.v1.proto
```

The exported proto will contain:
```proto
option csharp_namespace = "Example.Api";
```

## Best Practices

### 1. Go Package Format

For Go, use the semicolon format to specify both import path and package name:
```
github.com/org/project/gen;packagename
```

### 2. Consistent Naming

Maintain consistency across languages:
- Go: `github.com/example/api;apiv1`
- Java: `com.example.api`
- C#: `Example.Api`
- PHP: `Example\\Api`

### 3. Version in Package Names

Include API version in package names:
- Go: `github.com/example/api/v1;apiv1`
- Java: `com.example.api.v1`
- C#: `Example.Api.V1`

### 4. Automation

Create scripts to automate the export and generation process:

```bash
#!/bin/bash
# export-and-generate.sh

# Export protos with language options
hyperway proto export \
  --endpoint http://localhost:8080 \
  --output ./proto \
  --go-package "github.com/example/api;apiv1" \
  --java-package "com.example.api"

# Generate Go code
buf generate

# Generate Java code
protoc --java_out=./java/src \
       --grpc-java_out=./java/src \
       proto/*.proto
```

## Migration from Manual Editing

If you were previously manually editing exported proto files to add language options, you can now:

1. **Update your export scripts** to use the new options:
   ```bash
   # Before (required manual editing)
   hyperway proto export --endpoint http://localhost:8080
   
   # After (no manual editing needed)
   hyperway proto export \
     --endpoint http://localhost:8080 \
     --go-package "github.com/example/api;apiv1"
   ```

2. **Update your service code** to export with options:
   ```go
   // Before
   files, err := svc.ExportAllProtos()
   // Then manually edit files...
   
   // After
   files, err := svc.ExportAllProtosWithOptions(
       proto.WithGoPackage("github.com/example/api;apiv1"),
   )
   // No manual editing needed!
   ```

## Conclusion

Language-specific options in Hyperway's proto export feature streamline the workflow from service definition to client code generation across multiple programming languages. This feature maintains Hyperway's philosophy of eliminating manual proto file management while ensuring full compatibility with standard protobuf tooling.