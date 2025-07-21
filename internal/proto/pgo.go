// Package proto provides internal protobuf compilation utilities.
package proto

import (
	"fmt"
	"sync"

	"buf.build/go/hyperpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PGOManager manages profile-guided optimization for hyperpb message types.
type PGOManager struct {
	mu       sync.RWMutex
	profiles map[string]*hyperpb.Profile     // MessageType full name -> Profile
	msgTypes map[string]*hyperpb.MessageType // MessageType full name -> MessageType
}

// NewPGOManager creates a new PGO manager.
func NewPGOManager() *PGOManager {
	return &PGOManager{
		profiles: make(map[string]*hyperpb.Profile),
		msgTypes: make(map[string]*hyperpb.MessageType),
	}
}

// GetOrCreateProfile gets an existing profile or creates a new one for the message type.
func (m *PGOManager) GetOrCreateProfile(msgType *hyperpb.MessageType) *hyperpb.Profile {
	fullName := string(msgType.Descriptor().FullName())

	m.mu.RLock()
	profile, exists := m.profiles[fullName]
	m.mu.RUnlock()

	if exists {
		return profile
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	profile, exists = m.profiles[fullName]
	if exists {
		return profile
	}

	// Create new profile
	profile = msgType.NewProfile()
	m.profiles[fullName] = profile
	m.msgTypes[fullName] = msgType

	return profile
}

// GetProfile returns the profile for the given message type, or nil if none exists.
func (m *PGOManager) GetProfile(fullName string) *hyperpb.Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.profiles[fullName]
}

// RecompileWithProfile recompiles the message type with its collected profile.
func (m *PGOManager) RecompileWithProfile(fullName string) (*hyperpb.MessageType, error) {
	m.mu.RLock()
	profile, hasProfile := m.profiles[fullName]
	msgType, hasMsgType := m.msgTypes[fullName]
	m.mu.RUnlock()

	if !hasProfile || !hasMsgType {
		return nil, fmt.Errorf("no profile found for message type %s", fullName)
	}

	// Recompile with profile
	optimized := msgType.Recompile(profile)

	// Update stored message type
	m.mu.Lock()
	m.msgTypes[fullName] = optimized
	m.mu.Unlock()

	return optimized, nil
}

// RecompileAll recompiles all message types with their collected profiles.
func (m *PGOManager) RecompileAll() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.profiles))
	for name := range m.profiles {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if _, err := m.RecompileWithProfile(name); err != nil {
			return fmt.Errorf("failed to recompile %s: %w", name, err)
		}
	}

	return nil
}

// GetOptimizedMessageType returns the optimized message type if available, or nil.
func (m *PGOManager) GetOptimizedMessageType(fullName string) *hyperpb.MessageType {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.msgTypes[fullName]
}

// CompileWithPGO compiles a message descriptor with PGO support.
func CompileWithPGO(md protoreflect.MessageDescriptor, pgoManager *PGOManager) (*hyperpb.MessageType, error) {
	fullName := string(md.FullName())

	// Check if we already have an optimized version
	if optimized := pgoManager.GetOptimizedMessageType(fullName); optimized != nil {
		return optimized, nil
	}

	// Otherwise compile normally
	msgType, err := CompileMessageType(md)
	if err != nil {
		return nil, err
	}

	// Store in PGO manager for future optimization
	pgoManager.mu.Lock()
	pgoManager.msgTypes[fullName] = msgType
	pgoManager.mu.Unlock()

	return msgType, nil
}

// GlobalPGOManager is the default PGO manager instance.
var GlobalPGOManager = NewPGOManager()
