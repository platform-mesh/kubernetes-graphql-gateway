package apischema

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func ResolveSchema(ctx context.Context, dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	resolver := NewResolver()
	return resolver.Resolve(ctx, dc, rm)
}

func GetOpenAPISchemaKey(gvk metav1.GroupVersionKind) string {
	return getOpenAPISchemaKey(gvk)
}

func (b *SchemaBuilder) GetSchemas() map[string]*spec.Schema {
	return b.schemas
}

func (b *SchemaBuilder) GetError() error {
	return b.err
}

func (b *SchemaBuilder) SetSchemas(schemas map[string]*spec.Schema) {
	b.schemas = schemas
}
