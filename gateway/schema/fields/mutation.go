package fields

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

type MutationGenerator struct {
	resolver resolver.Provider
}

func NewMutationGenerator(resolver resolver.Provider) *MutationGenerator {
	return &MutationGenerator{resolver: resolver}
}

func (g *MutationGenerator) Generate(rc *ResourceContext, target *graphql.Object) {
	itemArgsBuilder := resolver.NewFieldConfigArguments().WithName()
	creationArgsBuilder := resolver.NewFieldConfigArguments().WithObject(rc.InputType).WithDryRun()

	if rc.IsNamespaceScoped() {
		itemArgsBuilder.WithNamespace()
		creationArgsBuilder.WithNamespace()
	}

	target.AddFieldConfig("create"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    creationArgsBuilder.Complete(),
		Resolve: g.resolver.CreateItem(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig("update"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    creationArgsBuilder.WithName().Complete(),
		Resolve: g.resolver.UpdateItem(rc.GVK, rc.Scope),
	})

	target.AddFieldConfig("delete"+rc.SingularName, &graphql.Field{
		Type:    graphql.Boolean,
		Args:    itemArgsBuilder.WithDryRun().Complete(),
		Resolve: g.resolver.DeleteItem(rc.GVK, rc.Scope),
	})
}
