package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/registry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/watcher"
)

// Service orchestrates the domain-driven architecture with target clusters
type Service struct {
	clusterRegistry *registry.ClusterRegistry
	// TODO: This should be generalized to multiple watchers
	schemaWatcher *watcher.FileWatcher

	config *GatewayConfig

	started bool
}

type GatewayConfig struct {
	DevelopmentDisableAuth bool

	GraphQLPretty            bool
	GraphQLPlayground        bool
	GraphQLGraphiQL          bool
	ServerCORSAllowedOrigins []string
	ServerCORSAllowedHeaders []string

	SchemaDirectory string
}

// New creates a new domain-driven Gateway instance
func New(config GatewayConfig) (*Service, error) {
	clusterRegistry := registry.New(registry.ClusterRegistryConfig{
		SchemaDirectory:        config.SchemaDirectory,
		DevelopmentDisableAuth: config.DevelopmentDisableAuth,
		GraphQLPretty:          config.GraphQLPretty,
		GraphQLPlayground:      config.GraphQLPlayground,
		GraphQLGraphiQL:        config.GraphQLGraphiQL,
		ServerCORSConfig: registry.CORSConfig{
			AllowedOrigins: config.ServerCORSAllowedOrigins,
			AllowedHeaders: config.ServerCORSAllowedHeaders,
		},
	})

	// Cluster registry acts as main server and is fed into watcher. So watcher updated new clusters there.
	schemaWatcher, err := watcher.NewFileWatcher(clusterRegistry)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create schema watcher")
	}

	gateway := &Service{
		clusterRegistry: clusterRegistry,
		schemaWatcher:   schemaWatcher,
		config:          &config,
	}

	return gateway, nil
}

// Run starts the gateway service
func (g *Service) Run(ctx context.Context) error {
	// Initialize schema watcher with context
	if err := g.schemaWatcher.Initialize(ctx, g.config.SchemaDirectory); err != nil {
		return fmt.Errorf("failed to initialize schema watcher: %w", err)
	}
	g.started = true
	<-ctx.Done()
	return nil
}

// ServeHTTP delegates HTTP requests to the cluster registry
func (g *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !g.started {
		http.Error(w, "Gateway not started", http.StatusServiceUnavailable)
		return
	}
	// Delegate to cluster registry
	g.clusterRegistry.ServeHTTP(w, r)
}

// Shutdown gracefully shuts down the gateway and all its services
func (g *Service) Shutdown(ctx context.Context) error {
	if g.clusterRegistry != nil {
		return g.clusterRegistry.Close(ctx)
	}
	return nil
}
