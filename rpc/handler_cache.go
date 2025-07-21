package rpc

import (
	"fmt"
	"reflect"
	"sync"
)

// HandlerInfo holds pre-computed handler information
type HandlerInfo struct {
	HandlerValue reflect.Value
	HandlerType  reflect.Type
	InputType    reflect.Type
	OutputType   reflect.Type
	IsPointer    bool // Whether handler expects pointer input
}

// HandlerCache caches pre-computed handler information
type HandlerCache struct {
	mu    sync.RWMutex
	cache map[uintptr]*HandlerInfo
}

// globalHandlerCache is a singleton handler cache
var globalHandlerCache = &HandlerCache{
	cache: make(map[uintptr]*HandlerInfo),
}

// GetHandlerInfo returns cached handler information or computes it
func GetHandlerInfo(handler any) (*HandlerInfo, error) {
	// Get function pointer as key
	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler must be a function")
	}

	key := handlerValue.Pointer()

	// Try to get from cache
	globalHandlerCache.mu.RLock()
	if info, ok := globalHandlerCache.cache[key]; ok {
		globalHandlerCache.mu.RUnlock()
		return info, nil
	}
	globalHandlerCache.mu.RUnlock()

	// Compute handler info
	globalHandlerCache.mu.Lock()
	defer globalHandlerCache.mu.Unlock()

	// Double-check after acquiring write lock
	if info, ok := globalHandlerCache.cache[key]; ok {
		return info, nil
	}

	// Build handler info
	handlerType := handlerValue.Type()

	// Validate signature
	if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
		return nil, fmt.Errorf("handler must have signature func(context.Context, *Input) (*Output, error)")
	}

	inputType := handlerType.In(1)
	isPointer := inputType.Kind() == reflect.Ptr
	if isPointer {
		inputType = inputType.Elem()
	}

	outputType := handlerType.Out(0)
	if outputType.Kind() == reflect.Ptr {
		outputType = outputType.Elem()
	}

	info := &HandlerInfo{
		HandlerValue: handlerValue,
		HandlerType:  handlerType,
		InputType:    inputType,
		OutputType:   outputType,
		IsPointer:    isPointer,
	}

	globalHandlerCache.cache[key] = info
	return info, nil
}

// ClearCache clears the handler cache (useful for testing)
func ClearHandlerCache() {
	globalHandlerCache.mu.Lock()
	defer globalHandlerCache.mu.Unlock()
	globalHandlerCache.cache = make(map[uintptr]*HandlerInfo)
}
