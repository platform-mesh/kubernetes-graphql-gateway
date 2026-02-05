package apischema

import (
	"errors"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

var (
	// ErrInvalidGVKFormat indicates the x-kubernetes-group-version-kind extension has an unexpected format.
	ErrInvalidGVKFormat = errors.New("invalid GVK extension format")

	// ErrScopeNotFound indicates the x-kubernetes-scope extension is missing.
	ErrScopeNotFound = errors.New("scope extension not found")

	// ErrInvalidScopeFormat indicates the scope extension has an unexpected format.
	ErrInvalidScopeFormat = errors.New("invalid scope extension format")
)

// ExtractGVK extracts GVK from schema extensions using type assertion.
// Returns nil if schema has no GVK extension (e.g., sub-resources).
func ExtractGVK(schema *spec.Schema) (*GroupVersionKind, error) {
	if schema == nil || schema.Extensions == nil {
		return nil, nil
	}

	gvksVal, ok := schema.Extensions[apis.GVKExtensionKey]
	if !ok {
		return nil, nil
	}

	// Try direct type assertion path first (most common)
	if gvkSlice, ok := gvksVal.([]any); ok {
		return extractFromInterfaceSlice(gvkSlice)
	}

	// Fallback: might be []map[string]any already
	if mapSlice, ok := gvksVal.([]map[string]any); ok {
		return extractFromMapSlice(mapSlice)
	}

	return nil, ErrInvalidGVKFormat
}

func extractFromInterfaceSlice(slice []any) (*GroupVersionKind, error) {
	if len(slice) != 1 {
		return nil, nil // Skip schemas with multiple or zero GVKs
	}

	gvkMap, ok := slice[0].(map[string]any)
	if !ok {
		return nil, ErrInvalidGVKFormat
	}

	return gvkFromMap(gvkMap), nil
}

func extractFromMapSlice(slice []map[string]any) (*GroupVersionKind, error) {
	if len(slice) != 1 {
		return nil, nil
	}

	return gvkFromMap(slice[0]), nil
}

// gvkFromMap extracts a GroupVersionKind from a map with group/version/kind keys.
func gvkFromMap(m map[string]any) *GroupVersionKind {
	return &GroupVersionKind{
		Group:   mapValue[string](m, "group"),
		Version: mapValue[string](m, "version"),
		Kind:    mapValue[string](m, "kind"),
	}
}

// mapValue extracts a typed value from a map, returning the zero value if not found or wrong type.
func mapValue[T any](m map[string]any, key string) T {
	var zero T
	if v, ok := m[key]; ok {
		if typed, ok := v.(T); ok {
			return typed
		}
	}
	return zero
}

// ExtractScope extracts the resource scope from schema extensions.
// Returns the scope (Namespaced or Cluster) or an error if not found.
func ExtractScope(schema *spec.Schema) (apiextensionsv1.ResourceScope, error) {
	if schema == nil || schema.Extensions == nil {
		return "", ErrScopeNotFound
	}

	scopeRaw, ok := schema.Extensions[apis.ScopeExtensionKey]
	if !ok {
		return "", ErrScopeNotFound
	}

	// Handle both string and ResourceScope types
	switch v := scopeRaw.(type) {
	case string:
		return apiextensionsv1.ResourceScope(v), nil
	case apiextensionsv1.ResourceScope:
		return v, nil
	default:
		return "", ErrInvalidScopeFormat
	}
}

// HasGVK returns true if the schema has a valid GVK extension.
func HasGVK(schema *spec.Schema) bool {
	gvk, err := ExtractGVK(schema)
	return err == nil && gvk != nil && gvk.Kind != ""
}

// HasScope returns true if the schema has a scope extension.
func HasScope(schema *spec.Schema) bool {
	_, err := ExtractScope(schema)
	return err == nil
}
