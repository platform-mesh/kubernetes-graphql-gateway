package apischema

import (
	"github.com/platform-mesh/golang-commons/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func ResolveSchema(dc discovery.DiscoveryInterface, rm meta.RESTMapper, log *logger.Logger) ([]byte, error) {
	crdResolver := NewCRDResolver(dc, rm, log)
	return crdResolver.resolveSchema(dc, rm)
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
