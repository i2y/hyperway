package schema

import (
	"container/list"
	"sync"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Cache provides thread-safe caching for compiled descriptors.
type Cache interface {
	GetMessage(key string) (protoreflect.MessageDescriptor, bool)
	PutMessage(key string, md protoreflect.MessageDescriptor)
	GetFile(key string) (*descriptorpb.FileDescriptorProto, bool)
	PutFile(key string, fd *descriptorpb.FileDescriptorProto)
	Clear()
	Size() int
}

// LRUCache implements an LRU cache for descriptors.
type LRUCache struct {
	mu       sync.RWMutex
	maxSize  int
	messages map[string]*lruEntry
	files    map[string]*lruEntry
	lru      *list.List
}

type lruEntry struct {
	key   string
	value any
	elem  *list.Element
}

// NewLRUCache creates a new LRU cache with the specified max size.
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize:  maxSize,
		messages: make(map[string]*lruEntry),
		files:    make(map[string]*lruEntry),
		lru:      list.New(),
	}
}

// GetMessage retrieves a message descriptor from the cache.
func (c *LRUCache) GetMessage(key string) (protoreflect.MessageDescriptor, bool) {
	c.mu.RLock()
	entry, ok := c.messages[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	c.mu.Lock()
	c.lru.MoveToFront(entry.elem)
	c.mu.Unlock()

	return entry.value.(protoreflect.MessageDescriptor), true
}

// PutMessage stores a message descriptor in the cache.
func (c *LRUCache) PutMessage(key string, md protoreflect.MessageDescriptor) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.messages[key]; ok {
		entry.value = md
		c.lru.MoveToFront(entry.elem)

		return
	}

	// Add new entry
	entry := &lruEntry{key: key, value: md}
	elem := c.lru.PushFront(entry)
	entry.elem = elem
	c.messages[key] = entry

	// Evict if necessary
	if c.maxSize > 0 && c.lru.Len() > c.maxSize {
		c.evictOldest()
	}
}

// GetFile retrieves a file descriptor from the cache.
func (c *LRUCache) GetFile(key string) (*descriptorpb.FileDescriptorProto, bool) {
	c.mu.RLock()
	entry, ok := c.files[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	c.mu.Lock()
	c.lru.MoveToFront(entry.elem)
	c.mu.Unlock()

	return entry.value.(*descriptorpb.FileDescriptorProto), true
}

// PutFile stores a file descriptor in the cache.
func (c *LRUCache) PutFile(key string, fd *descriptorpb.FileDescriptorProto) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.files[key]; ok {
		entry.value = fd
		c.lru.MoveToFront(entry.elem)

		return
	}

	// Add new entry
	entry := &lruEntry{key: key, value: fd}
	elem := c.lru.PushFront(entry)
	entry.elem = elem
	c.files[key] = entry

	// Evict if necessary
	if c.maxSize > 0 && c.lru.Len() > c.maxSize {
		c.evictOldest()
	}
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = make(map[string]*lruEntry)
	c.files = make(map[string]*lruEntry)
	c.lru.Init()
}

// Size returns the current size of the cache.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lru.Len()
}

// evictOldest removes the least recently used entry.
func (c *LRUCache) evictOldest() {
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*lruEntry)
	c.lru.Remove(elem)

	// Remove from appropriate map
	delete(c.messages, entry.key)
	delete(c.files, entry.key)
}

const (
	// DefaultGlobalCacheSize is the default size for the global cache
	DefaultGlobalCacheSize = 1000
)

// GlobalCache is a singleton cache instance.
var globalCache Cache

var globalCacheOnce sync.Once

// GetGlobalCache returns the global cache instance.
func GetGlobalCache() Cache {
	globalCacheOnce.Do(func() {
		globalCache = NewLRUCache(DefaultGlobalCacheSize)
	})

	return globalCache
}

// SetGlobalCache sets a custom global cache.
func SetGlobalCache(cache Cache) {
	globalCache = cache
}
