package extensions

import (
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

const (
	typeByCategoryFieldName = "typeByCategory"
)

type CustomQueryGenerator struct {
	resolver        resolver.Provider
	categoryManager *CategoryManager
}

func NewCustomQueryGenerator(resolver resolver.Provider, categoryManager *CategoryManager) *CustomQueryGenerator {
	return &CustomQueryGenerator{
		resolver:        resolver,
		categoryManager: categoryManager,
	}
}

func (g *CustomQueryGenerator) AddTypeByCategoryQuery(rootQueryFields graphql.Fields) {
	resourceType := graphql.NewObject(graphql.ObjectConfig{
		Name: typeByCategoryFieldName + "Object",
		Fields: graphql.Fields{
			"kind":    graphqlStringField(),
			"group":   graphqlStringField(),
			"version": graphqlStringField(),
			"scope":   graphqlStringField(),
		},
	})

	rootQueryFields[typeByCategoryFieldName] = &graphql.Field{
		Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(resourceType))),
		Args: resolver.NewFieldConfigArguments().
			WithName().
			Complete(),
		Resolve: g.resolver.TypeByCategory(g.categoryManager.AllCategories()),
	}
}

func graphqlStringField() *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewNonNull(graphql.String),
	}
}
