package codec

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	reflectutil "github.com/i2y/hyperway/internal/reflect"
)

// StructEncoder provides struct to protobuf encoding.
type StructEncoder struct {
	descriptor protoreflect.MessageDescriptor
}

// NewStructEncoder creates a new struct encoder.
func NewStructEncoder(md protoreflect.MessageDescriptor) *StructEncoder {
	return &StructEncoder{
		descriptor: md,
	}
}

// EncodeStruct encodes a Go struct directly to protobuf binary.
func (se *StructEncoder) EncodeStruct(source any) ([]byte, error) {
	// Create a dynamic message that supports Set operations
	msg := dynamicpb.NewMessage(se.descriptor)

	// Convert struct to proto message directly
	if err := reflectutil.StructToProto(source, msg.ProtoReflect()); err != nil {
		return nil, fmt.Errorf("failed to convert struct to proto: %w", err)
	}

	// Marshal to binary
	return proto.Marshal(msg)
}
