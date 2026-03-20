package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/registry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/watcher"
	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Service orchestrates the gateway with target clusters.
type Service struct {
	registry *registry.Registry
	config   config.Gateway
	started  bool
	ready    chan struct{}
	readyOnce sync.Once
}

// New creates a new Gateway service.
func New(cfg config.Gateway) (*Service, error) {
	return &Service{
		registry: registry.New(cfg),
		config:   cfg,
		ready:    make(chan struct{}),
	}, nil
}

// Run starts the gateway service with the configured watcher.
func (s *Service) Run(ctx context.Context) error {
	logger := log.FromContext(ctx)
	s.started = true
	s.readyOnce.Do(func() { close(s.ready) })

	switch s.config.SchemaHandler {
	case "file":
		logger.Info("Starting file watcher", "directory", s.config.SchemaDirectory)
		fw, err := watcher.NewFileWatcher(s.registry)
		if err != nil {
			return fmt.Errorf("failed to create file watcher: %w", err)
		}
		return fw.Run(ctx, s.config.SchemaDirectory)

	case "grpc":
		logger.Info("Starting gRPC watcher", "address", s.config.GRPCAddress)
		gw, err := watcher.NewGRPCWatcher(
			watcher.GRPCWatcherConfig{Address: s.config.GRPCAddress},
			s.registry,
		)
		if err != nil {
			return fmt.Errorf("failed to create gRPC watcher: %w", err)
		}
		return gw.Run(ctx)

	default:
		return fmt.Errorf("unknown schema handler: %s", s.config.SchemaHandler)
	}
}

// ServeHTTP routes HTTP requests to the appropriate endpoint.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context())

	if !s.started {
		http.Error(w, "Gateway not started", http.StatusServiceUnavailable)
		return
	}

	// Extract cluster name from context (set by HTTP mux from path parameter)
	clusterName, ok := utilscontext.GetClusterFromCtx(r.Context())
	if !ok || clusterName == "" {
		logger.Error(fmt.Errorf("cluster name not found in context"), "Missing cluster name", "path", r.URL.Path)
		http.Error(w, "Cluster name is required in path: /api/clusters/{clusterName}", http.StatusBadRequest)
		return
	}

	// Get endpoint for cluster
	endpoint, exists := s.registry.GetEndpoint(clusterName)
	if !exists {
		logger.Error(fmt.Errorf("endpoint not found"), "Target endpoint not found",
			"cluster", clusterName,
			"path", r.URL.Path,
		)
		http.NotFound(w, r)
		return
	}

	logger.V(4).Info("Routing request to endpoint",
		"cluster", clusterName,
		"method", r.Method,
		"path", r.URL.Path,
	)

	endpoint.ServeHTTP(w, r)
}

// Registry returns the endpoint registry for direct access if needed.
func (s *Service) Registry() *registry.Registry {
	return s.registry
}

// WaitForReady blocks until the gateway service has started or the context is cancelled.
func (s *Service) WaitForReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
