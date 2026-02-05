package fields

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

// QueryGenerator generates GraphQL query fields for a resource.
type QueryGenerator struct {
	resolver resolver.Provider
}

// NewQueryGenerator creates a new query field generator.
func NewQueryGenerator(resolver resolver.Provider) *QueryGenerator {
	return &QueryGenerator{resolver: resolver}
}

// Generate adds query fields (list, get, getYaml) to the target object.
func (g *QueryGenerator) Generate(rc *ResourceContext, target *graphql.Object) {
	listArgsBuilder := resolver.NewFieldConfigArguments().
		WithLabelSelector().
		WithSortBy().
		WithLimit().
		WithContinue()

	itemArgsBuilder := resolver.NewFieldConfigArguments().WithName()

	if rc.IsNamespaceScoped() {
		listArgsBuilder.WithNamespace()
		itemArgsBuilder.WithNamespace()
	}

	listArgs := listArgsBuilder.Complete()
	itemArgs := itemArgsBuilder.Complete()

	// Create list wrapper type
	listWrapperType := graphql.NewObject(graphql.ObjectConfig{
		Name: rc.UniqueTypeName + "List",
		Fields: graphql.Fields{
			"resourceVersion":    &graphql.Field{Type: graphql.String},
			"items":              &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(rc.ResourceType)))},
			"continue":           &graphql.Field{Type: graphql.String},
			"remainingItemCount": &graphql.Field{Type: graphql.Int},
		},
	})

	// Add list query
	target.AddFieldConfig(rc.PluralName, &graphql.Field{
		Type:    graphql.NewNonNull(listWrapperType),
		Args:    listArgs,
		Resolve: g.resolver.ListItems(rc.Ctx, rc.GVK, rc.Scope),
	})

	// Add get query
	target.AddFieldConfig(rc.SingularName, &graphql.Field{
		Type:    graphql.NewNonNull(rc.ResourceType),
		Args:    itemArgs,
		Resolve: g.resolver.GetItem(rc.Ctx, rc.GVK, rc.Scope),
	})

	// Add getYaml query
	target.AddFieldConfig(rc.SingularName+"Yaml", &graphql.Field{
		Type:    graphql.NewNonNull(graphql.String),
		Args:    itemArgs,
		Resolve: g.resolver.GetItemAsYAML(rc.Ctx, rc.GVK, rc.Scope),
	})
}
