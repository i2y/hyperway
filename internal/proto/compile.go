// Package proto provides internal protobuf compilation utilities.
package proto

import (
	"fmt"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// CompileMessageType compiles a message descriptor into a hyperpb MessageType.
func CompileMessageType(md protoreflect.MessageDescriptor) (*hyperpb.MessageType, error) {
	return CompileMessageTypeWithOptions(md, nil)
}

// CompileMessageTypeWithOptions compiles a message descriptor into a hyperpb MessageType with options.
func CompileMessageTypeWithOptions(md protoreflect.MessageDescriptor, opts []hyperpb.CompileOption) (*hyperpb.MessageType, error) {
	// Create a FileDescriptorSet containing this message
	fdset := &descriptorpb.FileDescriptorSet{}

	// Add the file containing this message
	file := md.ParentFile()
	fdProto := protodesc.ToFileDescriptorProto(file)
	fdset.File = append(fdset.File, fdProto)

	// Add any dependencies
	for i := 0; i < file.Imports().Len(); i++ {
		imp := file.Imports().Get(i)
		impProto := protodesc.ToFileDescriptorProto(imp)
		fdset.File = append(fdset.File, impProto)
	}

	// Compile the message type
	msgType, err := hyperpb.CompileFileDescriptorSet(fdset, md.FullName(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to compile message type %s: %w", md.FullName(), err)
	}

	return msgType, nil
}

// CompileMessageTypeWithCache compiles a message descriptor with caching support.
func CompileMessageTypeWithCache(md protoreflect.MessageDescriptor, cache MessageTypeCache) (*hyperpb.MessageType, error) {
	key := string(md.FullName())

	// Check cache
	if msgType, ok := cache.Get(key); ok {
		return msgType, nil
	}

	// Compile
	msgType, err := CompileMessageType(md)
	if err != nil {
		return nil, err
	}

	// Cache the result
	cache.Put(key, msgType)

	return msgType, nil
}

// MessageTypeCache defines the interface for caching compiled message types.
type MessageTypeCache interface {
	Get(key string) (*hyperpb.MessageType, bool)
	Put(key string, msgType *hyperpb.MessageType)
}
