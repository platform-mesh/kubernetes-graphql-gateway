package registry

import (
	"context"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/endpoint"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Registry manages multiple endpoints (cluster + GraphQL handler pairs).
type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]*endpoint.Endpoint
	config    config.Gateway
}

// New creates a new endpoint registry.
func New(cfg config.Gateway) *Registry {
	return &Registry{
		endpoints: make(map[string]*endpoint.Endpoint),
		config:    cfg,
	}
}

// OnSchemaChanged implements watcher.SchemaEventHandler.
// It is called when a schema is created or updated.
func (r *Registry) OnSchemaChanged(ctx context.Context, clusterName string, schema string) {
	logger := log.FromContext(ctx)
	logger.V(4).Info("Loading endpoint", "cluster", clusterName)

	// Create endpoint outside the lock to avoid holding it during slow operations
	ep, err := endpoint.New(
		ctx,
		clusterName,
		schema,
		r.config.GraphQL,
	)
	if err != nil {
		logger.Error(err, "Failed to create endpoint", "cluster", clusterName)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[clusterName]; exists {
		logger.V(4).Info("Replaced existing endpoint", "cluster", clusterName)
	}

	r.endpoints[clusterName] = ep
	logger.Info("Successfully loaded endpoint", "cluster", clusterName)
}

// OnSchemaDeleted implements watcher.SchemaEventHandler.
// It is called when a schema is removed.
func (r *Registry) OnSchemaDeleted(ctx context.Context, clusterName string) {
	logger := log.FromContext(ctx)

	r.mu.Lock()
	defer r.mu.Unlock()

	logger.V(4).Info("Removing endpoint", "cluster", clusterName)

	if _, exists := r.endpoints[clusterName]; !exists {
		logger.V(2).Info("Attempted to remove non-existent endpoint", "cluster", clusterName)
		return
	}

	delete(r.endpoints, clusterName)
	logger.Info("Successfully removed endpoint", "cluster", clusterName)
}

// GetEndpoint returns an endpoint by cluster name.
func (r *Registry) GetEndpoint(name string) (*endpoint.Endpoint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ep, exists := r.endpoints[name]
	return ep, exists
}
