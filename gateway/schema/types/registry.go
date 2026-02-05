package types

import (
	"sync"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConversionState tracks the state of type conversion to handle recursion properly.
type ConversionState int

const (
	// StateNotStarted indicates type conversion has not begun.
	StateNotStarted ConversionState = iota
	// StateProcessing indicates type is currently being converted (recursion guard).
	StateProcessing
	// StateComplete indicates type conversion is finished.
	StateComplete
)

// TypeEntry holds both output and input types for a schema key.
type TypeEntry struct {
	Output *graphql.Object
	Input  *graphql.InputObject
	State  ConversionState
}

// Registry provides unified type storage with O(1) lookups.
// It replaces the scattered caches (typesCache, inputTypesCache, typeNameRegistry)
// with a single coherent type management system.
type Registry struct {
	mu sync.RWMutex

	// types stores GraphQL types by their schema key
	types map[string]*TypeEntry

	// typeNameRegistry tracks kind names to GroupVersion mappings for conflict detection
	typeNameRegistry map[string]string

	// sanitizeGroupFn is used to sanitize group names for GraphQL compatibility
	sanitizeGroupFn func(string) string
}

// NewRegistry creates a new TypeRegistry with the given group sanitization function.
func NewRegistry(sanitizeGroupFn func(string) string) *Registry {
	return &Registry{
		types:            make(map[string]*TypeEntry),
		typeNameRegistry: make(map[string]string),
		sanitizeGroupFn:  sanitizeGroupFn,
	}
}

// Register stores output and input types by key.
func (r *Registry) Register(key string, output *graphql.Object, input *graphql.InputObject) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateEntry(key)
	entry.Output = output
	entry.Input = input
	entry.State = StateComplete
}

// Get retrieves types by key. Returns nil values if not found or not complete.
func (r *Registry) Get(key string) (*graphql.Object, *graphql.InputObject) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	if !exists || entry.State != StateComplete {
		return nil, nil
	}
	return entry.Output, entry.Input
}

// GetOutput retrieves only the output type by key.
func (r *Registry) GetOutput(key string) *graphql.Object {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	if !exists {
		return nil
	}
	return entry.Output
}

// GetInput retrieves only the input type by key.
func (r *Registry) GetInput(key string) *graphql.InputObject {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	if !exists {
		return nil
	}
	return entry.Input
}

// IsProcessing checks if a type is currently being processed (recursion guard).
func (r *Registry) IsProcessing(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	return exists && entry.State == StateProcessing
}

// MarkProcessing marks a type as being processed to prevent infinite recursion.
func (r *Registry) MarkProcessing(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateEntry(key)
	entry.State = StateProcessing
}

// UnmarkProcessing removes the processing mark (used with defer after MarkProcessing).
func (r *Registry) UnmarkProcessing(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.types[key]; exists {
		// Only unmark if still processing (not completed)
		if entry.State == StateProcessing {
			entry.State = StateNotStarted
		}
	}
}

// GetState returns the current conversion state for a key.
func (r *Registry) GetState(key string) ConversionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	if !exists {
		return StateNotStarted
	}
	return entry.State
}

// GetUniqueTypeName returns a unique type name for a GVK, handling conflicts
// when the same Kind exists in different API groups.
func (r *Registry) GetUniqueTypeName(gvk *schema.GroupVersionKind) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	kind := gvk.Kind
	groupVersion := gvk.GroupVersion().String()

	// Check if the kind name has already been used for a different group/version
	if existingGroupVersion, exists := r.typeNameRegistry[kind]; exists {
		if existingGroupVersion != groupVersion {
			// Conflict detected, append group and version to the kind for uniqueness
			sanitizedGroup := ""
			if gvk.Group != "" && r.sanitizeGroupFn != nil {
				sanitizedGroup = r.sanitizeGroupFn(gvk.Group)
			}
			return flect.Pascalize(sanitizedGroup+"_"+gvk.Version) + kind
		}
	} else {
		// No conflict, register the kind with its group and version
		r.typeNameRegistry[kind] = groupVersion
	}

	return kind
}

// HasType checks if a type entry exists for the given key.
func (r *Registry) HasType(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.types[key]
	return exists
}

// getOrCreateEntry gets or creates a TypeEntry for the given key.
// Must be called with lock held.
func (r *Registry) getOrCreateEntry(key string) *TypeEntry {
	entry, exists := r.types[key]
	if !exists {
		entry = &TypeEntry{State: StateNotStarted}
		r.types[key] = entry
	}
	return entry
}
