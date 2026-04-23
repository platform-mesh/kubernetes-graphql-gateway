package types

import (
	"regexp"
	"sync"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	invalidGroupCharRegex = regexp.MustCompile(`[^_a-zA-Z0-9]`)
	validGroupStartRegex  = regexp.MustCompile(`^[_a-zA-Z]`)
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
	mu    sync.RWMutex
	types map[string]*TypeEntry
}

func NewRegistry() *Registry {
	return &Registry{
		types: make(map[string]*TypeEntry),
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

// GetUniqueTypeName returns a fully-qualified type name for a GVK by always
// prefixing with group+version. This prevents collisions between nested type
// names (e.g., Component + status → ComponentStatus) and resource type names
// (e.g., the built-in ComponentStatus Kind).
func (r *Registry) GetUniqueTypeName(gvk *schema.GroupVersionKind) string {
	sanitizedGroup := ""
	if gvk.Group != "" {
		sanitizedGroup = SanitizeGroupName(gvk.Group)
	}
	return flect.Pascalize(sanitizedGroup+"_"+gvk.Version) + gvk.Kind
}

// SanitizeGroupName converts a Kubernetes API group name to a valid GraphQL identifier.
// It replaces invalid characters with underscores and ensures the name starts with a letter or underscore.
func SanitizeGroupName(groupName string) string {
	sanitized := invalidGroupCharRegex.ReplaceAllString(groupName, "_")
	if sanitized != "" && !validGroupStartRegex.MatchString(sanitized) {
		sanitized = "_" + sanitized
	}
	return sanitized
}

func (r *Registry) getOrCreateEntry(key string) *TypeEntry {
	entry, exists := r.types[key]
	if !exists {
		entry = &TypeEntry{State: StateNotStarted}
		r.types[key] = entry
	}
	return entry
}
