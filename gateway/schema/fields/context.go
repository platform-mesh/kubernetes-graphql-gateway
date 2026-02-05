package fields

import (
	"github.com/graphql-go/graphql"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceContext struct {
	GVK            schema.GroupVersionKind
	Scope          apiextensionsv1.ResourceScope
	UniqueTypeName string
	ResourceType   *graphql.Object
	InputType      *graphql.InputObject
	SingularName   string
	PluralName     string
	SanitizedGroup string
}

func (r *ResourceContext) IsNamespaceScoped() bool {
	return r.Scope == apiextensionsv1.NamespaceScoped
}

func (r *ResourceContext) IsRootGroup() bool {
	return r.GVK.Group == ""
}
