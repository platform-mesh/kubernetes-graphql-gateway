package apischema

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func GetCRDGroupKindVersions(spec apiextensionsv1.CustomResourceDefinitionSpec) *GroupKindVersions {
	return getCRDGroupKindVersions(spec)
}

func IsCRDKindIncluded(gkv *GroupKindVersions, apiList *metav1.APIResourceList) bool {
	return isCRDKindIncluded(gkv, apiList)
}

func ErrorIfCRDNotInPreferredApiGroups(gkv *GroupKindVersions, lists []*metav1.APIResourceList) ([]string, error) {
	return errorIfCRDNotInPreferredApiGroups(gkv, lists, nil)
}

func GetSchemaForPath(preferred []string, path string, gv openapi.GroupVersion) (map[string]*spec.Schema, error) {
	return getSchemaForPath(preferred, path, gv)
}

func ResolveSchema(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	return resolveSchema(dc, rm)
}

func GetOpenAPISchemaKey(gvk metav1.GroupVersionKind) string {
	return getOpenAPISchemaKey(gvk)
}

func GetCRDGroupVersionKind(spec apiextensionsv1.CustomResourceDefinitionSpec) (*metav1.GroupVersionKind, error) {
	return getCRDGroupVersionKind(spec)
}

type (
	SchemaResponse           = schemaResponse
	SchemasComponentsWrapper = schemasComponentsWrapper
)

func (b *SchemaBuilder) GetSchemas() map[string]*spec.Schema {
	return b.schemas
}

func (b *SchemaBuilder) GetError() error {
	return b.err
}

func (b *SchemaBuilder) SetSchemas(schemas map[string]*spec.Schema) {
	b.schemas = schemas
}
