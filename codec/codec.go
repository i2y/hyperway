// Package codec provides high-performance encoding/decoding using hyperpb.
package codec

import (
	"fmt"

	"buf.build/go/hyperpb"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Constants for buffer and pool sizes
const (
	initialBufSize = 10
)

// Codec provides high-level encoding/decoding operations.
type Codec struct {
	encoder       *Encoder
	decoder       *Decoder
	structEncoder *StructEncoder
}

// Options configures codec behavior.
type Options struct {
	// EnablePooling enables message pooling
	EnablePooling bool
	// PoolSize sets the initial pool size
	PoolSize int
	// AllowUnknownFields allows unknown fields when decoding
	AllowUnknownFields bool
}

// DefaultOptions returns default codec options.
func DefaultOptions() Options {
	return Options{
		EnablePooling:      true,
		PoolSize:           initialBufSize,
		AllowUnknownFields: false,
	}
}

// New creates a new codec for the given message descriptor.
func New(md protoreflect.MessageDescriptor, opts Options) (*Codec, error) {
	encoder, err := NewEncoder(md, EncoderOptions{
		EnablePooling:   opts.EnablePooling,
		InitialPoolSize: opts.PoolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder: %w", err)
	}

	decoder, err := NewDecoder(md, DecoderOptions{
		EnablePooling:      opts.EnablePooling,
		InitialPoolSize:    opts.PoolSize,
		AllowUnknownFields: opts.AllowUnknownFields,
		EnablePGO:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	structEncoder := NewStructEncoder(md)

	return &Codec{
		encoder:       encoder,
		decoder:       decoder,
		structEncoder: structEncoder,
	}, nil
}

// Marshal encodes a message to bytes.
func (c *Codec) Marshal(msg protobuf.Message) ([]byte, error) {
	return c.encoder.Encode(msg)
}

// Unmarshal decodes bytes to a message.
func (c *Codec) Unmarshal(data []byte) (protobuf.Message, error) {
	return c.decoder.Decode(data)
}

// MarshalToJSON encodes a message to JSON.
func (c *Codec) MarshalToJSON(msg protobuf.Message) ([]byte, error) {
	return c.encoder.EncodeJSON(msg)
}

// UnmarshalFromJSON decodes JSON to a message.
func (c *Codec) UnmarshalFromJSON(data []byte) (protobuf.Message, error) {
	return c.decoder.DecodeJSON(data)
}

// NewMessage creates a new message instance.
func (c *Codec) NewMessage() protobuf.Message {
	return c.decoder.GetMessage()
}

// ReleaseMessage returns a message to the pool.
func (c *Codec) ReleaseMessage(msg protobuf.Message) {
	// Only release hyperpb messages
	if hm, ok := msg.(*hyperpb.Message); ok {
		c.decoder.PutMessage(hm)
	}
}

// Descriptor returns the message descriptor.
func (c *Codec) Descriptor() protoreflect.MessageDescriptor {
	return c.encoder.Descriptor()
}

// MarshalStruct encodes a Go struct directly to protobuf binary.
func (c *Codec) MarshalStruct(source any) ([]byte, error) {
	return c.structEncoder.EncodeStruct(source)
}
