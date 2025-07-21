package codec

import (
	"sync"
	"sync/atomic"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/i2y/hyperway/internal/proto"
)

// MessagePool manages a pool of reusable messages.
type MessagePool struct {
	descriptor protoreflect.MessageDescriptor
	msgType    *hyperpb.MessageType
	pool       sync.Pool
	stats      PoolStats
}

// PoolStats tracks pool usage statistics.
type PoolStats struct {
	Gets    atomic.Int64
	Puts    atomic.Int64
	News    atomic.Int64
	InUse   atomic.Int64
	MaxSize atomic.Int64
}

// NewMessagePool creates a new message pool.
func NewMessagePool(md protoreflect.MessageDescriptor) (*MessagePool, error) {
	// Compile the message type
	msgType, err := proto.CompileMessageType(md)
	if err != nil {
		return nil, err
	}

	mp := &MessagePool{
		descriptor: md,
		msgType:    msgType,
	}
	mp.pool.New = func() any {
		mp.stats.News.Add(1)

		return hyperpb.NewMessage(msgType)
	}

	return mp, nil
}

// Get retrieves a message from the pool.
func (p *MessagePool) Get() *hyperpb.Message {
	p.stats.Gets.Add(1)
	p.stats.InUse.Add(1)

	// Update max size if needed
	for {
		current := p.stats.InUse.Load()
		maxSize := p.stats.MaxSize.Load()

		if current <= maxSize || p.stats.MaxSize.CompareAndSwap(maxSize, current) {
			break
		}
	}

	return p.pool.Get().(*hyperpb.Message)
}

// Put returns a message to the pool.
func (p *MessagePool) Put(msg *hyperpb.Message) {
	if msg == nil {
		return
	}

	msg.Reset()
	p.stats.Puts.Add(1)
	p.stats.InUse.Add(-1)
	p.pool.Put(msg)
}

// Stats returns the current pool statistics.
func (p *MessagePool) Stats() PoolStats {
	return PoolStats{
		Gets:    atomic.Int64{},
		Puts:    atomic.Int64{},
		News:    atomic.Int64{},
		InUse:   atomic.Int64{},
		MaxSize: atomic.Int64{},
	}
}

// GlobalPools manages pools for all message types.
type GlobalPools struct {
	mu    sync.RWMutex
	pools map[protoreflect.MessageDescriptor]*MessagePool
}

var globalPools = &GlobalPools{
	pools: make(map[protoreflect.MessageDescriptor]*MessagePool),
}

// GetPool returns a pool for the given message descriptor.
func GetPool(md protoreflect.MessageDescriptor) (*MessagePool, error) {
	globalPools.mu.RLock()
	pool, ok := globalPools.pools[md]
	globalPools.mu.RUnlock()

	if ok {
		return pool, nil
	}

	globalPools.mu.Lock()
	defer globalPools.mu.Unlock()

	// Double-check after acquiring write lock
	pool, ok = globalPools.pools[md]
	if ok {
		return pool, nil
	}

	var err error

	pool, err = NewMessagePool(md)
	if err != nil {
		return nil, err
	}

	globalPools.pools[md] = pool

	return pool, nil
}

// ClearPools clears all global pools.
func ClearPools() {
	globalPools.mu.Lock()
	defer globalPools.mu.Unlock()
	globalPools.pools = make(map[protoreflect.MessageDescriptor]*MessagePool)
}
