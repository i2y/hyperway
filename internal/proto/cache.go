package proto

import (
	"sync"

	"buf.build/go/hyperpb"
)

// SimpleCache is a thread-safe cache for compiled message types.
type SimpleCache struct {
	mu    sync.RWMutex
	cache map[string]*hyperpb.MessageType
}

// NewSimpleCache creates a new simple cache.
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		cache: make(map[string]*hyperpb.MessageType),
	}
}

// Get retrieves a message type from the cache.
func (c *SimpleCache) Get(key string) (*hyperpb.MessageType, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	msgType, ok := c.cache[key]

	return msgType, ok
}

// Put stores a message type in the cache.
func (c *SimpleCache) Put(key string, msgType *hyperpb.MessageType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = msgType
}

// Clear removes all entries from the cache.
func (c *SimpleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*hyperpb.MessageType)
}

// Size returns the number of entries in the cache.
func (c *SimpleCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

var globalCache = NewSimpleCache()

// GetGlobalCache returns the global message type cache.
func GetGlobalCache() MessageTypeCache {
	return globalCache
}
