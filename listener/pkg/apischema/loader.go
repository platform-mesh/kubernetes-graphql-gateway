package apischema

import (
	"context"
	"encoding/json"
	"errors"
	"maps"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/schemamutation"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SchemaLoader loads OpenAPI schemas from a Kubernetes API server.
type SchemaLoader struct{}

// NewSchemaLoader creates a new SchemaLoader.
func NewSchemaLoader() *SchemaLoader {
	return &SchemaLoader{}
}

// Load fetches and parses all OpenAPI schemas from the client.
// GVK is extracted once per schema via type assertion
func (l *SchemaLoader) Load(ctx context.Context, oc openapi.Client) (*SchemaSet, error) {
	logger := log.FromContext(ctx)

	paths, err := oc.Paths()
	if err != nil {
		return nil, errors.Join(ErrGetOpenAPIPaths, err)
	}

	entries := make(map[string]*SchemaEntry)
	walker := createRefWalker()

	for pathKey, path := range paths {
		pathEntries, errs := l.loadPath(ctx, path, walker)
		for _, e := range errs {
			logger.V(4).Info("error loading schema path",
				"path", pathKey,
				"error", e)
		}

		maps.Copy(entries, pathEntries)
	}

	logger.Info("loaded schemas", "count", len(entries))

	return NewSchemaSet(entries), nil
}

func (l *SchemaLoader) loadPath(
	ctx context.Context,
	path openapi.GroupVersion,
	walker schemamutation.Walker,
) (map[string]*SchemaEntry, []error) {
	logger := log.FromContext(ctx)
	entries := make(map[string]*SchemaEntry)
	var errs []error

	schemaBytes, err := path.Schema(discovery.AcceptV2)
	if err != nil {
		errs = append(errs, err)
		return entries, errs
	}

	var openAPISpec spec3.OpenAPI
	if err := json.Unmarshal(schemaBytes, &openAPISpec); err != nil {
		errs = append(errs, err)
		return entries, errs
	}

	if openAPISpec.Components == nil {
		return entries, errs
	}

	for key, schema := range openAPISpec.Components.Schemas {
		// Walk and normalize refs
		walked := walker.WalkSchema(schema)

		gvk, err := extractGVK(walked)
		if err != nil {
			logger.V(4).Info("failed to extract GVK",
				"key", key,
				"error", err)
			errs = append(errs, err)
			continue
		}

		entries[key] = &SchemaEntry{
			Key:    key,
			Schema: walked,
			GVK:    gvk,
		}
	}

	return entries, errs
}

// extractGVK extracts GVK from schema extensions using type assertion.
// Returns nil if schema has no GVK extension (e.g., sub-resources).
func extractGVK(schema *spec.Schema) (*GroupVersionKind, error) {
	if schema.Extensions == nil {
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

// createRefWalker creates a schema walker that normalizes $ref pointers.
// This simplifies refs from full paths to short names.
func createRefWalker() schemamutation.Walker {
	return schemamutation.Walker{
		RefCallback: schemamutation.RefCallbackNoop,
		SchemaCallback: func(schema *spec.Schema) *spec.Schema {
			refPtr := schema.Ref.GetPointer()
			if refPtr == nil {
				return schema
			}

			tokens := refPtr.DecodedTokens()
			if len(tokens) == 0 {
				return schema
			}

			resolvedRef := tokens[len(tokens)-1]
			return &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Ref: spec.MustCreateRef(resolvedRef),
				},
			}
		},
	}
}
