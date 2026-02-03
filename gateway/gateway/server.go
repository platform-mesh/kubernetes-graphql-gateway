package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/registry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/watcher"
)

// Service orchestrates the domain-driven architecture with target clusters
type Service struct {
	clusterRegistry *registry.ClusterRegistry
	config          *GatewayConfig

	started bool
}

// GatewayConfig holds configuration for the gateway service.
type GatewayConfig struct {
	DevelopmentDisableAuth bool

	GraphQLPretty     bool
	GraphQLPlayground bool
	GraphQLGraphiQL   bool

	// SchemaHandler specifies which watcher to use ("file" or "grpc")
	SchemaHandler string

	// SchemaDirectory is used when SchemaHandler is "file"
	SchemaDirectory string

	// GRPCAddress is used when SchemaHandler is "grpc"
	GRPCAddress string
}

// New creates a new domain-driven Gateway instance
func New(config GatewayConfig) (*Service, error) {
	clusterRegistry := registry.New(registry.ClusterRegistryConfig{
		DevelopmentDisableAuth: config.DevelopmentDisableAuth,
		GraphQLPretty:          config.GraphQLPretty,
		GraphQLPlayground:      config.GraphQLPlayground,
		GraphQLGraphiQL:        config.GraphQLGraphiQL,
	})

	return &Service{
		clusterRegistry: clusterRegistry,
		config:          &config,
	}, nil
}

// Run starts the gateway service with the configured watcher.
func (g *Service) Run(ctx context.Context) error {
	g.started = true

	switch g.config.SchemaHandler {
	case "file":
		fw, err := watcher.NewFileWatcher(g.clusterRegistry)
		if err != nil {
			return fmt.Errorf("failed to create file watcher: %w", err)
		}
		return fw.Run(ctx, g.config.SchemaDirectory)

	case "grpc":
		gw, err := watcher.NewGRPCWatcher(
			watcher.GRPCWatcherConfig{Address: g.config.GRPCAddress},
			g.clusterRegistry,
		)
		if err != nil {
			return fmt.Errorf("failed to create gRPC watcher: %w", err)
		}
		return gw.Run(ctx)

	default:
		return fmt.Errorf("unknown schema handler: %s", g.config.SchemaHandler)
	}
}

// ServeHTTP delegates HTTP requests to the cluster registry
func (g *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !g.started {
		http.Error(w, "Gateway not started", http.StatusServiceUnavailable)
		return
	}
	g.clusterRegistry.ServeHTTP(w, r)
}
