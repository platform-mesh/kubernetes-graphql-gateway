package manager

import (
	"fmt"
	"net/http"

	"github.com/openmfp/golang-commons/logger"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"

	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/roundtripper"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/targetcluster"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/watcher"
)

// Service orchestrates the domain-driven architecture with target clusters
type Service struct {
	log             *logger.Logger
	clusterRegistry ClusterManager
	schemaWatcher   SchemaWatcher
}

// NewGateway creates a new domain-driven Gateway instance
func NewGateway(log *logger.Logger, appCfg appConfig.Config) (*Service, error) {
	// Create round tripper factory
	roundTripperFactory := targetcluster.RoundTripperFactory(func(adminRT http.RoundTripper, tlsConfig rest.TLSClientConfig) http.RoundTripper {
		return roundtripper.New(log, appCfg, adminRT, roundtripper.NewUnauthorizedRoundTripper())
	})

	clusterRegistry := targetcluster.NewClusterRegistry(log, appCfg, roundTripperFactory)

	schemaWatcher, err := watcher.NewFileWatcher(log, clusterRegistry)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create schema watcher")
	}

	gateway := &Service{
		log:             log,
		clusterRegistry: clusterRegistry,
		schemaWatcher:   schemaWatcher,
	}

	// Initialize schema watcher
	if err := schemaWatcher.Initialize(appCfg.OpenApiDefinitionsPath); err != nil {
		return nil, fmt.Errorf("failed to initialize schema watcher: %w", err)
	}

	log.Info().
		Str("definitions_path", appCfg.OpenApiDefinitionsPath).
		Str("port", appCfg.Gateway.Port).
		Msg("Gateway initialized successfully")

	return gateway, nil
}

// ServeHTTP delegates HTTP requests to the cluster registry
func (g *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.clusterRegistry.ServeHTTP(w, r)
}

// Close gracefully shuts down the gateway and all its services
func (g *Service) Close() error {
	if g.schemaWatcher != nil {
		g.schemaWatcher.Close()
	}
	if g.clusterRegistry != nil {
		g.clusterRegistry.Close()
	}
	g.log.Info().Msg("The Gateway has been closed")
	return nil
}
