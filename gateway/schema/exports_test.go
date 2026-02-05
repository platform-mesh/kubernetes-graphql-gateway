package schema

import (
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var StringMapScalarForTest = types.StringMapScalar
var JSONStringScalarForTest = types.JSONStringScalar

func GetGatewayForTest() *Gateway {
	registry := types.NewRegistry(func(s string) string { return s })
	return &Gateway{
		typeRegistry: registry,
	}
}

func (g *Gateway) GetNamesForTest(gvk *schema.GroupVersionKind) (singular, plural string) {
	return g.getNames(gvk)
}

func GenerateTypeNameForTest(typePrefix string, fieldPath []string) string {
	return types.GenerateTypeName(typePrefix, fieldPath)
}

func SanitizeFieldNameForTest(name string) string {
	return types.SanitizeFieldName(name)
}
