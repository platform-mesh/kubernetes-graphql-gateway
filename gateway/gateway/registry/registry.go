package registry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/platform-mesh/golang-commons/sentry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/cluster"
	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterRegistryConfig holds configuration for the ClusterRegistry.
type ClusterRegistryConfig struct {
	DevelopmentDisableAuth bool

	GraphQLPretty     bool
	GraphQLPlayground bool
	GraphQLGraphiQL   bool
}

// ClusterRegistry manages multiple target clusters and handles HTTP routing to them
type ClusterRegistry struct {
	mu       sync.RWMutex
	clusters map[string]*cluster.Cluster
	config   ClusterRegistryConfig
}

// New creates a new cluster registry
func New(
	config ClusterRegistryConfig,
) *ClusterRegistry {
	return &ClusterRegistry{
		clusters: make(map[string]*cluster.Cluster),
		config:   config,
	}
}

// OnSchemaChanged implements watcher.SchemaEventHandler.
// It is called when a schema is created or updated.
func (cr *ClusterRegistry) OnSchemaChanged(ctx context.Context, clusterName string, schema string) {
	logger := log.FromContext(ctx)

	cr.mu.Lock()
	defer cr.mu.Unlock()

	logger.V(4).WithValues("cluster", clusterName).Info("Loading target cluster")

	// Remove existing cluster if present
	if _, exists := cr.clusters[clusterName]; exists {
		delete(cr.clusters, clusterName)
		logger.V(4).WithValues("cluster", clusterName).Info("Removed existing cluster for update")
	}

	// Create new cluster from schema
	cl, err := cluster.New(clusterName, schema, cluster.ClusterConfig{
		DevelopmentDisableAuth: cr.config.DevelopmentDisableAuth,
		GraphQLPretty:          cr.config.GraphQLPretty,
		GraphQLPlayground:      cr.config.GraphQLPlayground,
		GraphQLGraphiQL:        cr.config.GraphQLGraphiQL,
	})
	if err != nil {
		logger.Error(err, "Failed to create cluster", "cluster", clusterName)
		sentry.CaptureError(err, sentry.Tags{"cluster": clusterName})
		return
	}

	cr.clusters[clusterName] = cl
	logger.Info("Successfully loaded cluster", "cluster", clusterName)
}

// OnSchemaDeleted implements watcher.SchemaEventHandler.
// It is called when a schema is removed.
func (cr *ClusterRegistry) OnSchemaDeleted(ctx context.Context, clusterName string) {
	logger := log.FromContext(ctx)

	cr.mu.Lock()
	defer cr.mu.Unlock()

	logger.V(4).WithValues("cluster", clusterName).Info("Removing target cluster")

	_, exists := cr.clusters[clusterName]
	if !exists {
		logger.V(2).WithValues("cluster", clusterName).Info("Attempted to remove non-existent cluster")
		return
	}

	delete(cr.clusters, clusterName)
	logger.Info("Successfully removed cluster", "cluster", clusterName)
}

// GetCluster returns a cluster by name
func (cr *ClusterRegistry) GetCluster(name string) (*cluster.Cluster, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	cluster, exists := cr.clusters[name]
	return cluster, exists
}

// ServeHTTP routes HTTP requests to the appropriate target cluster
func (cr *ClusterRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context())
	// Extract cluster name from context (set by HTTP mux from path parameter)
	clusterName, ok := utilscontext.GetClusterFromCtx(r.Context())
	if !ok || clusterName == "" {
		logger.WithValues("path", r.URL.Path).Error(fmt.Errorf("cluster name not found in context"), "Missing cluster name")
		http.Error(w, "Cluster name is required in path: /api/clusters/{clusterName}", http.StatusBadRequest)
		return
	}

	// Get target cluster
	cluster, exists := cr.GetCluster(clusterName)
	if !exists {
		logger.WithValues(
			"cluster", clusterName,
			"path", r.URL.Path,
		).Error(fmt.Errorf("cluster not found"), "Target cluster not found")
		http.NotFound(w, r)
		return
	}

	// Handle GET requests (GraphiQL/Playground) directly
	if r.Method == http.MethodGet {
		cluster.ServeHTTP(w, r)
		return
	}

	// Route to target cluster
	logger.V(4).WithValues(
		"cluster", clusterName,
		"method", r.Method,
		"path", r.URL.Path,
	).Info("Routing request to target cluster")

	cluster.ServeHTTP(w, r)
}
