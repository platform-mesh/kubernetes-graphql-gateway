package fields

import (
	"context"

	"github.com/graphql-go/graphql"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceContext provides all context needed for field generation.
// It contains pre-computed values to avoid redundant calculations across generators.
type ResourceContext struct {
	Ctx context.Context

	// GVK is the GroupVersionKind of the resource
	GVK schema.GroupVersionKind

	// Scope indicates if resource is namespace-scoped or cluster-scoped
	Scope apiextensionsv1.ResourceScope

	// UniqueTypeName is the GraphQL type name (handles naming conflicts)
	UniqueTypeName string

	// ResourceType is the GraphQL object type for the resource
	ResourceType *graphql.Object

	// InputType is the GraphQL input object type for mutations
	InputType *graphql.InputObject

	// SingularName is the singular form of the resource name (e.g., "Pod")
	SingularName string

	// PluralName is the plural form of the resource name (e.g., "Pods")
	PluralName string

	// SanitizedGroup is the GraphQL-safe group name (empty for core API)
	SanitizedGroup string
}

// IsNamespaceScoped returns true if the resource is namespace-scoped.
func (r *ResourceContext) IsNamespaceScoped() bool {
	return r.Scope == apiextensionsv1.NamespaceScoped
}

// IsRootGroup returns true if the resource belongs to the core API group.
func (r *ResourceContext) IsRootGroup() bool {
	return r.GVK.Group == ""
}
