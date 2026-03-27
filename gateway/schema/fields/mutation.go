package fields

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

type MutationGenerator struct {
	resolver *resolver.Service
}

func NewMutationGenerator(resolver *resolver.Service) *MutationGenerator {
	return &MutationGenerator{resolver: resolver}
}

func (g *MutationGenerator) Generate(rc *ResourceContext, target *graphql.Object) {
	target.AddFieldConfig("create"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    resolver.CreateArgs(rc.Scope, rc.InputType),
		Resolve: g.resolver.CreateItem(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig("update"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    resolver.UpdateArgs(rc.Scope, rc.InputType),
		Resolve: g.resolver.UpdateItem(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig("delete"+rc.SingularName, &graphql.Field{
		Type:    graphql.Boolean,
		Args:    resolver.DeleteArgs(rc.Scope),
		Resolve: g.resolver.DeleteItem(rc.GVK, rc.Scope),
	})
}
