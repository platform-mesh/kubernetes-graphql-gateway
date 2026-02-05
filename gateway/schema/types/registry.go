package types

import (
	"sync"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ConversionState int

const (
	StateNotStarted ConversionState = iota
	StateProcessing
	StateComplete
)

type TypeEntry struct {
	Output *graphql.Object
	Input  *graphql.InputObject
	State  ConversionState
}

type Registry struct {
	mu               sync.RWMutex
	types            map[string]*TypeEntry
	typeNameRegistry map[string]string
	sanitizeGroupFn  func(string) string
}

func NewRegistry(sanitizeGroupFn func(string) string) *Registry {
	return &Registry{
		types:            make(map[string]*TypeEntry),
		typeNameRegistry: make(map[string]string),
		sanitizeGroupFn:  sanitizeGroupFn,
	}
}

func (r *Registry) Register(key string, output *graphql.Object, input *graphql.InputObject) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateEntry(key)
	entry.Output = output
	entry.Input = input
	entry.State = StateComplete
}

func (r *Registry) Get(key string) (*graphql.Object, *graphql.InputObject) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	if !exists || entry.State != StateComplete {
		return nil, nil
	}
	return entry.Output, entry.Input
}

func (r *Registry) IsProcessing(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.types[key]
	return exists && entry.State == StateProcessing
}

func (r *Registry) MarkProcessing(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateEntry(key)
	entry.State = StateProcessing
}

func (r *Registry) UnmarkProcessing(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.types[key]; exists {
		if entry.State == StateProcessing {
			entry.State = StateNotStarted
		}
	}
}

// GetUniqueTypeName returns a unique type name for a GVK, handling conflicts
// when the same Kind exists in different API groups.
func (r *Registry) GetUniqueTypeName(gvk *schema.GroupVersionKind) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	kind := gvk.Kind
	groupVersion := gvk.GroupVersion().String()

	if existingGroupVersion, exists := r.typeNameRegistry[kind]; exists {
		if existingGroupVersion != groupVersion {
			sanitizedGroup := ""
			if gvk.Group != "" && r.sanitizeGroupFn != nil {
				sanitizedGroup = r.sanitizeGroupFn(gvk.Group)
			}
			return flect.Pascalize(sanitizedGroup+"_"+gvk.Version) + kind
		}
	} else {
		r.typeNameRegistry[kind] = groupVersion
	}

	return kind
}

func (r *Registry) getOrCreateEntry(key string) *TypeEntry {
	entry, exists := r.types[key]
	if !exists {
		entry = &TypeEntry{State: StateNotStarted}
		r.types[key] = entry
	}
	return entry
}
