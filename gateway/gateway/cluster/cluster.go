package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterConfig struct {
	DevelopmentDisableAuth bool

	GraphQLPretty     bool
	GraphQLPlayground bool
	GraphQLGraphiQL   bool
}

// TargetCluster represents a single target Kubernetes cluster
type Cluster struct {
	config ClusterConfig

	name          string
	client        client.WithWatch
	restCfg       *rest.Config
	handler       *graphql.GraphQLHandler
	graphqlServer *graphql.GraphQLServer
}

// New creates a new Cluster from a schema file
func New(
	name string,
	schemaFilePath string,
	config ClusterConfig,
) (*Cluster, error) {
	fileData, err := readSchemaFile(schemaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	cluster := &Cluster{
		name: name,
	}

	// Connect to cluster - use metadata if available, otherwise fall back to standard config
	if err := cluster.connectAndSetClient(config, fileData.ClusterMetadata); err != nil {
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}

	// Create GraphQL schema and handler
	// Create resolver
	resolverProvider := resolver.New(cluster.client)

	// Create schema gateway
	schemaGateway, err := schema.New(fileData.Components.Schemas, resolverProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL schema: %w", err)
	}

	// Create and store GraphQL server and handler
	cluster.graphqlServer = graphql.NewGraphQLServer(graphql.GraphQLConfig{
		Pretty:     config.GraphQLPretty,
		Playground: config.GraphQLPlayground,
		GraphiQL:   config.GraphQLGraphiQL,
	})
	cluster.handler = cluster.graphqlServer.CreateHandler(schemaGateway.GetSchema())

	log.Info().
		Str("cluster", name).
		Msg("Registered endpoint")

	return cluster, nil
}

// connectAndSetClient establishes connection to the target cluster
func (tc *Cluster) connectAndSetClient(config ClusterConfig, metadata *v1alpha1.ClusterMetadata) error {
	// All clusters now use metadata from schema files to get kubeconfig
	if metadata == nil {
		return fmt.Errorf("cluster %s requires cluster metadata in schema file", tc.name)
	}

	var err error
	tc.restCfg, err = v1alpha1.BuildRestConfigFromMetadata(*metadata)
	if err != nil {
		return fmt.Errorf("failed to build config from metadata: %w", err)
	}

	baseRT, err := roundtripper.NewBaseRoundTripper(tc.restCfg.TLSClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create base transport: %w", err)
	}

	// TODO: this should be somehow middleware, not roundtripper.
	tc.restCfg.Wrap(func(adminRT http.RoundTripper) http.RoundTripper {
		return roundtripper.New(
			adminRT,
			baseRT,
			roundtripper.NewUnauthorizedRoundTripper(),
			config.DevelopmentDisableAuth,
		)
	})

	// Create client - in the new multicluster architecture, the cluster context
	// is handled via context.WithCluster, so we use the standard client for both modes
	tc.client, err = client.NewWithWatch(tc.restCfg, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create cluster client: %w", err)
	}

	return nil
}

// ServeHTTP handles HTTP requests for this cluster
func (tc *Cluster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if tc.handler == nil || tc.handler.Handler == nil {
		http.Error(w, "Cluster not ready", http.StatusServiceUnavailable)
		return
	}

	// Handle subscription requests using Server-Sent Events
	if r.Header.Get("Accept") == "text/event-stream" {
		tc.graphqlServer.HandleSubscription(w, r, tc.handler.Schema)
		return
	}

	tc.handler.Handler.ServeHTTP(w, r)
}

// readSchemaFile reads and parses a schema file
func readSchemaFile(filePath string) (*v1alpha1.Schema, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var fileData v1alpha1.Schema
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &fileData, nil
}
