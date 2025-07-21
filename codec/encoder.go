package codec

import (
	"fmt"
	"sync"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/encoding/protojson"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/i2y/hyperway/internal/proto"
)

// Encoder handles encoding Go values to protobuf format using hyperpb.
type Encoder struct {
	msgType    *hyperpb.MessageType
	descriptor protoreflect.MessageDescriptor
	pool       *sync.Pool
	options    EncoderOptions
}

// EncoderOptions configures the encoder.
type EncoderOptions struct {
	// EnablePooling enables message pooling for better performance
	EnablePooling bool
	// InitialPoolSize sets the initial pool size
	InitialPoolSize int
}

// NewEncoder creates a new encoder for the given message descriptor.
func NewEncoder(md protoreflect.MessageDescriptor, opts EncoderOptions) (*Encoder, error) {
	// Compile the message type
	msgType, err := proto.CompileMessageType(md)
	if err != nil {
		return nil, fmt.Errorf("failed to compile message type: %w", err)
	}

	enc := &Encoder{
		msgType:    msgType,
		descriptor: md,
		options:    opts,
	}

	if opts.EnablePooling {
		enc.pool = &sync.Pool{
			New: func() any {
				return hyperpb.NewMessage(msgType)
			},
		}
		// Pre-populate pool
		for i := 0; i < opts.InitialPoolSize; i++ {
			enc.pool.Put(hyperpb.NewMessage(msgType))
		}
	}

	return enc, nil
}

// Encode marshals a protobuf message to bytes.
func (e *Encoder) Encode(msg protobuf.Message) ([]byte, error) {
	// Use standard protobuf marshaling
	return protobuf.Marshal(msg)
}

// EncodeJSON marshals a protobuf message to JSON.
func (e *Encoder) EncodeJSON(msg protobuf.Message) ([]byte, error) {
	// Convert to JSON using protojson
	return protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}.Marshal(msg)
}

// GetMessage returns a message from the pool or creates a new one.
func (e *Encoder) GetMessage() *hyperpb.Message {
	if e.pool != nil {
		msg, ok := e.pool.Get().(*hyperpb.Message)
		if ok {
			return msg
		}
	}

	return hyperpb.NewMessage(e.msgType)
}

// PutMessage returns a message to the pool.
func (e *Encoder) PutMessage(msg *hyperpb.Message) {
	if e.pool != nil {
		msg.Reset()
		e.pool.Put(msg)
	}
}

// Descriptor returns the message descriptor.
func (e *Encoder) Descriptor() protoreflect.MessageDescriptor {
	return e.descriptor
}
