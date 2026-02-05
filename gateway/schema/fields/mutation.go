package fields

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

// MutationGenerator generates GraphQL mutation fields for a resource.
type MutationGenerator struct {
	resolver resolver.Provider
}

// NewMutationGenerator creates a new mutation field generator.
func NewMutationGenerator(resolver resolver.Provider) *MutationGenerator {
	return &MutationGenerator{resolver: resolver}
}

// Generate adds mutation fields (create, update, delete) to the target object.
func (g *MutationGenerator) Generate(rc *ResourceContext, target *graphql.Object) {
	itemArgsBuilder := resolver.NewFieldConfigArguments().WithName()
	creationArgsBuilder := resolver.NewFieldConfigArguments().WithObject(rc.InputType).WithDryRun()

	if rc.IsNamespaceScoped() {
		itemArgsBuilder.WithNamespace()
		creationArgsBuilder.WithNamespace()
	}

	// Add create mutation
	target.AddFieldConfig("create"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    creationArgsBuilder.Complete(),
		Resolve: g.resolver.CreateItem(rc.Ctx, rc.GVK, rc.Scope),
	})

	// Add update mutation
	target.AddFieldConfig("update"+rc.SingularName, &graphql.Field{
		Type:    rc.ResourceType,
		Args:    creationArgsBuilder.WithName().Complete(),
		Resolve: g.resolver.UpdateItem(rc.Ctx, rc.GVK, rc.Scope),
	})

	// Add delete mutation
	target.AddFieldConfig("delete"+rc.SingularName, &graphql.Field{
		Type:    graphql.Boolean,
		Args:    itemArgsBuilder.WithDryRun().Complete(),
		Resolve: g.resolver.DeleteItem(rc.Ctx, rc.GVK, rc.Scope),
	})
}
