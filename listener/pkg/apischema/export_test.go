package apischema

import (
	"context"

	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// ResolveSchema is a convenience function for tests.
func ResolveSchema(ctx context.Context, oc openapi.Client, enrichers ...Enricher) ([]byte, error) {
	resolver := NewResolver(enrichers...)
	return resolver.Resolve(ctx, oc)
}

// LoadSchemas loads schemas from an OpenAPI client for testing.
func LoadSchemas(ctx context.Context, oc openapi.Client) (*SchemaSet, error) {
	loader := NewSchemaLoader()
	return loader.Load(ctx, oc)
}

// ExtractGVKFromSchema extracts the GVK from a schema for testing.
func ExtractGVKFromSchema(schema *spec.Schema) (*GroupVersionKind, error) {
	return extractGVK(schema)
}
