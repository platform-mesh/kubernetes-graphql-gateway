package apischema

import (
	"encoding/json"
	"strings"

	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// GroupVersionKind represents a Kubernetes API group, version, and kind.
// Parsed once at load time and reused throughout the pipeline.
type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
}

// ToRuntimeGVK converts to k8s.io/apimachinery runtime.schema.GroupVersionKind
func (gvk GroupVersionKind) ToRuntimeGVK() runtimeSchema.GroupVersionKind {
	return runtimeSchema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	}
}

// SchemaEntry holds a schema with its parsed GVK metadata.
// Created once during loading - GVK never needs re-parsing.
type SchemaEntry struct {
	Key    string            // OpenAPI schema key (e.g., "io.k8s.api.core.v1.Pod")
	Schema *spec.Schema      // The actual OpenAPI schema
	GVK    *GroupVersionKind // Parsed GVK (nil for sub-resources)
}

// SchemaSet is an immutable collection of schemas with O(1) lookups.
type SchemaSet struct {
	entries map[string]*SchemaEntry           // key -> entry
	byKind  map[string][]*SchemaEntry         // lowercase(kind) -> entries
	byGVK   map[GroupVersionKind]*SchemaEntry // exact GVK -> entry
}

// NewSchemaSet creates a new SchemaSet from a map of entries.
// Builds secondary indexes for O(1) lookups.
func NewSchemaSet(entries map[string]*SchemaEntry) *SchemaSet {
	s := &SchemaSet{
		entries: entries,
		byKind:  make(map[string][]*SchemaEntry),
		byGVK:   make(map[GroupVersionKind]*SchemaEntry),
	}

	for _, entry := range entries {
		if entry.GVK == nil {
			continue
		}

		// Index by lowercase kind for O(1) lookup
		kindKey := strings.ToLower(entry.GVK.Kind)
		s.byKind[kindKey] = append(s.byKind[kindKey], entry)

		// Index by exact GVK
		s.byGVK[*entry.GVK] = entry
	}

	return s
}

// Get returns a schema entry by its key.
func (s *SchemaSet) Get(key string) (*SchemaEntry, bool) {
	entry, ok := s.entries[key]
	return entry, ok
}

// GetByGVK returns a schema entry by its exact GVK - O(1).
func (s *SchemaSet) GetByGVK(gvk GroupVersionKind) (*SchemaEntry, bool) {
	entry, ok := s.byGVK[gvk]
	return entry, ok
}

// FindByKind returns all schema entries matching a kind name - O(1).
// Kind matching is case-insensitive.
func (s *SchemaSet) FindByKind(kind string) []*SchemaEntry {
	return s.byKind[strings.ToLower(kind)]
}

// All returns all schema entries.
func (s *SchemaSet) All() map[string]*SchemaEntry {
	return s.entries
}

// Size returns the number of schema entries.
func (s *SchemaSet) Size() int {
	return len(s.entries)
}

// Marshal serializes the SchemaSet to OpenAPI v3 JSON.
func (s *SchemaSet) Marshal() ([]byte, error) {
	schemas := make(map[string]*spec.Schema, len(s.entries))
	for key, entry := range s.entries {
		schemas[key] = entry.Schema
	}

	return json.Marshal(&spec3.OpenAPI{
		Components: &spec3.Components{
			Schemas: schemas,
		},
	})
}

// NewSchemaSetFromMap creates a SchemaSet from raw schemas.
// Useful for testing - extracts GVK from each schema's extensions.
func NewSchemaSetFromMap(schemas map[string]*spec.Schema) *SchemaSet {
	entries := make(map[string]*SchemaEntry, len(schemas))
	for k, v := range schemas {
		gvk, _ := extractGVK(v)
		entries[k] = &SchemaEntry{
			Key:    k,
			Schema: v,
			GVK:    gvk,
		}
	}
	return NewSchemaSet(entries)
}
