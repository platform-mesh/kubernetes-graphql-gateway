package fields

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

type QueryGenerator struct {
	resolver *resolver.Service
}

func NewQueryGenerator(resolver *resolver.Service) *QueryGenerator {
	return &QueryGenerator{resolver: resolver}
}

func (g *QueryGenerator) Generate(rc *ResourceContext, target *graphql.Object) {
	listArgs := resolver.ListArgs(rc.Scope)
	itemArgs := resolver.ItemArgs(rc.Scope)

	listWrapperType := graphql.NewObject(graphql.ObjectConfig{
		Name:   rc.UniqueTypeName + "List",
		Fields: resolver.ListResultFields(rc.ResourceType),
	})

	target.AddFieldConfig(rc.PluralName, &graphql.Field{
		Type:    graphql.NewNonNull(listWrapperType),
		Args:    listArgs,
		Resolve: g.resolver.ListItems(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig(rc.SingularName, &graphql.Field{
		Type:    graphql.NewNonNull(rc.ResourceType),
		Args:    itemArgs,
		Resolve: g.resolver.GetItem(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig(rc.SingularName+"Yaml", &graphql.Field{
		Type:    graphql.NewNonNull(graphql.String),
		Args:    itemArgs,
		Resolve: g.resolver.GetItemAsYAML(rc.GVK, rc.Scope),
	})
}
