package schema

import (
	"github.com/graphql-go/graphql"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
)

const (
	typeByCategory = "typeByCategory"
)

func (g *Gateway) AddTypeByCategoryQuery(rootQueryFields graphql.Fields) {
	resourceType := graphql.NewObject(graphql.ObjectConfig{
		Name: typeByCategory + "Object",
		Fields: graphql.Fields{
			"kind":    graphqlStringField(),
			"group":   graphqlStringField(),
			"version": graphqlStringField(),
			"scope":   graphqlStringField(),
		},
	})

	rootQueryFields[typeByCategory] = &graphql.Field{
		Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(resourceType))),
		Args: resolver.NewFieldConfigArguments().
			WithNameArg().
			Complete(),
		Resolve: g.resolver.TypeByCategory(g.typeByCategory),
	}
}

func graphqlStringField() *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewNonNull(graphql.String),
	}
}
