package schema

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/builder"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

// Provider is the interface for accessing the GraphQL schema.
type Provider interface {
	GetSchema() *graphql.Schema
}

// Gateway provides access to the generated GraphQL schema.
// It acts as a thin facade over the builder package.
type Gateway struct {
	schema *graphql.Schema
}

// New creates a new Gateway with a GraphQL schema built from OpenAPI definitions.
func New(definitions map[string]*spec.Schema, resolverProvider resolver.Provider) (*Gateway, error) {
	b := builder.New(definitions, resolverProvider)

	schema, err := b.Build(context.TODO())
	if err != nil {
		return nil, err
	}

	return &Gateway{schema: schema}, nil
}

// GetSchema returns the generated GraphQL schema.
func (g *Gateway) GetSchema() *graphql.Schema {
	return g.schema
}
