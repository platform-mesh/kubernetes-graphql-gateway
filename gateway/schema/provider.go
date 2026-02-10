package schema

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/builder"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

// Provider provides access to the generated GraphQL schema.
// It acts as a thin facade over the builder package.
type Provider struct {
	schema *graphql.Schema
}

// New creates a new Provider with a GraphQL schema built from OpenAPI definitions.
func New(ctx context.Context, definitions map[string]*spec.Schema, resolverProvider resolver.Provider) (*Provider, error) {
	schema, err := builder.New(definitions, resolverProvider).Build(ctx)
	if err != nil {
		return nil, err
	}

	return &Provider{schema: schema}, nil
}

// GetSchema returns the generated GraphQL schema.
func (p *Provider) GetSchema() *graphql.Schema {
	return p.schema
}
