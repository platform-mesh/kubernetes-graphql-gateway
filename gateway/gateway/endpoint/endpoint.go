package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/cluster"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Endpoint combines a cluster connection with its GraphQL handler.
// It represents a complete, servable GraphQL endpoint for a Kubernetes cluster.
type Endpoint struct {
	name          string
	cluster       *cluster.Cluster
	graphqlServer *graphql.GraphQLServer
	handler       *graphql.GraphQLHandler
}

// New creates a new Endpoint from a schema JSON string.
func New(
	ctx context.Context,
	name string,
	schemaJSON string,
	graphqlCfg config.GraphQL,
) (*Endpoint, error) {
	schemaData, err := parseSchema(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Create cluster connection
	cl, err := cluster.New(ctx, name, schemaData.ClusterMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	// Create GraphQL schema and handler
	resolverProvider := resolver.New(cl.Client())

	schemaProvider, err := schema.New(ctx, schemaData.Components.Schemas, resolverProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL schema: %w", err)
	}

	// Create GraphQL server and handler
	graphqlServer := graphql.NewGraphQLServer(graphqlCfg)
	handler := graphqlServer.CreateHandler(schemaProvider.GetSchema())

	logger := log.FromContext(ctx)
	logger.Info("Registered endpoint", "cluster", name)

	return &Endpoint{
		name:          name,
		cluster:       cl,
		graphqlServer: graphqlServer,
		handler:       handler,
	}, nil
}

// ServeHTTP handles HTTP requests for this endpoint.
func (e *Endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e.handler == nil || e.handler.Handler == nil {
		http.Error(w, "Endpoint not ready", http.StatusServiceUnavailable)
		return
	}

	// Handle subscription requests using Server-Sent Events
	if r.Header.Get("Accept") == "text/event-stream" {
		e.graphqlServer.HandleSubscription(w, r, e.handler.Schema)
		return
	}

	e.handler.Handler.ServeHTTP(w, r)
}

// Name returns the endpoint name.
func (e *Endpoint) Name() string {
	return e.name
}

// parseSchema parses a JSON schema string into a Schema struct.
func parseSchema(schemaJSON string) (*v1alpha1.Schema, error) {
	var schemaData v1alpha1.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &schemaData, nil
}
