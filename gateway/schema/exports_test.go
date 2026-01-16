package schema

import (
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var StringMapScalarForTest = stringMapScalar
var JSONStringScalarForTest = jsonStringScalar

func GetGatewayForTest(typeNameRegistry map[string]string, resolverProvider resolver.Provider) *Gateway {
	return &Gateway{
		typeNameRegistry: typeNameRegistry,
		resolver:         resolverProvider,
	}
}

func (g *Gateway) GetNamesForTest(gvk *schema.GroupVersionKind) (singular, plural string) {
	return g.getNames(gvk)
}

func (g *Gateway) GetUniqueTypeNameForTest(gvk *schema.GroupVersionKind) string {
	return g.getUniqueTypeName(gvk)
}

func (g *Gateway) GenerateTypeNameForTest(typePrefix string, fieldPath []string) string {
	return g.generateTypeName(typePrefix, fieldPath)
}

func SanitizeFieldNameForTest(name string) string {
	return sanitizeFieldName(name)
}
