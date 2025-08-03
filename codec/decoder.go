package codec

import (
	"fmt"
	"sync"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/encoding/protojson"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/i2y/hyperway/internal/proto"
)

// Default pool size
const defaultPoolSize = 10

// Decoder handles decoding protobuf format to Go values using hyperpb.
type Decoder struct {
	msgType    *hyperpb.MessageType
	descriptor protoreflect.MessageDescriptor
	pool       *sync.Pool
	options    DecoderOptions
}

// DecoderOptions configures the decoder.
type DecoderOptions struct {
	// EnablePooling enables message pooling for better performance
	EnablePooling bool
	// InitialPoolSize sets the initial pool size
	InitialPoolSize int
	// AllowUnknownFields allows unknown fields in the input
	AllowUnknownFields bool
	// EnablePGO enables profile-guided optimization
	EnablePGO bool
}

// NewDecoder creates a new decoder for the given message descriptor.
func NewDecoder(md protoreflect.MessageDescriptor, opts DecoderOptions) (*Decoder, error) {
	var msgType *hyperpb.MessageType
	var err error

	if opts.EnablePGO {
		// Try to get optimized message type from PGO manager
		msgType, err = proto.CompileWithPGO(md, proto.GlobalPGOManager)
		if err != nil {
			return nil, fmt.Errorf("failed to compile message type with PGO: %w", err)
		}
	} else {
		// Compile normally
		msgType, err = proto.CompileMessageType(md)
		if err != nil {
			return nil, fmt.Errorf("failed to compile message type: %w", err)
		}
	}

	dec := &Decoder{
		msgType:    msgType,
		descriptor: md,
		options:    opts,
	}

	if opts.EnablePooling {
		dec.pool = &sync.Pool{
			New: func() any {
				return hyperpb.NewMessage(msgType)
			},
		}
		// Pre-populate pool
		for i := 0; i < opts.InitialPoolSize; i++ {
			dec.pool.Put(hyperpb.NewMessage(msgType))
		}
	}

	return dec, nil
}

// Decode unmarshals bytes to a protobuf message.
func (d *Decoder) Decode(data []byte) (*hyperpb.Message, error) {
	msg := d.GetMessage()

	// Get profile if PGO is enabled
	var unmarshalOpts []hyperpb.UnmarshalOption
	if d.options.EnablePGO {
		profile := proto.GlobalPGOManager.GetOrCreateProfile(d.msgType)
		if profile != nil {
			// Record with 10% sampling rate by default to minimize overhead
			const defaultSamplingRate = 0.1
			unmarshalOpts = append(unmarshalOpts, hyperpb.WithRecordProfile(profile, defaultSamplingRate))
		}
	}

	err := msg.Unmarshal(data, unmarshalOpts...)
	if err != nil {
		d.PutMessage(msg) // Return to pool on error

		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return msg, nil
}

// DecodeInto unmarshals bytes into an existing message.
func (d *Decoder) DecodeInto(data []byte, msg protobuf.Message) error {
	// Use standard proto unmarshal
	opts := protobuf.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: !d.options.AllowUnknownFields,
	}

	return opts.Unmarshal(data, msg)
}

// DecodeJSON unmarshals JSON to a protobuf message.
func (d *Decoder) DecodeJSON(data []byte) (*hyperpb.Message, error) {
	// Since hyperpb messages are read-only, we can't unmarshal JSON directly.
	// We need to use dynamicpb as an intermediate step
	dynamicMsg := dynamicpb.NewMessage(d.descriptor)

	// Convert from JSON using protojson
	opts := protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: !d.options.AllowUnknownFields,
	}

	err := opts.Unmarshal(data, dynamicMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Marshal to binary protobuf
	protoData, err := protobuf.Marshal(dynamicMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to protobuf: %w", err)
	}

	// Now unmarshal with hyperpb
	return d.Decode(protoData)
}

// GetMessage returns a message from the pool or creates a new one.
func (d *Decoder) GetMessage() *hyperpb.Message {
	// hyperpb messages are read-only and don't support Clear/Reset
	// So we don't use pooling for now
	// TODO: Implement pooling when hyperpb supports it
	return hyperpb.NewMessage(d.msgType)
}

// PutMessage returns a message to the pool.
func (d *Decoder) PutMessage(msg *hyperpb.Message) {
	// hyperpb messages are read-only and don't support Clear/Reset
	// So we don't use pooling for now
	// TODO: Implement pooling when hyperpb supports it
}

// Descriptor returns the message descriptor.
func (d *Decoder) Descriptor() protoreflect.MessageDescriptor {
	return d.descriptor
}

// DefaultDecoderOptions are the default options for decoders.
var DefaultDecoderOptions = DecoderOptions{
	EnablePooling:      true,
	InitialPoolSize:    defaultPoolSize,
	AllowUnknownFields: false,
	EnablePGO:          false,
}
