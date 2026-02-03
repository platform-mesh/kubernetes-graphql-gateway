package registry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/cluster"
	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterRegistryConfig holds configuration for the ClusterRegistry.
// TODO: Move this into options for dedicated gateway config.
type ClusterRegistryConfig struct {
	SchemaDirectory        string
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

// LoadCluster loads a target cluster from a schema file
func (cr *ClusterRegistry) LoadCluster(ctx context.Context, schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.loadCluster(ctx, schemaFilePath)
}

// loadCluster loads a target cluster from a schema file
func (cr *ClusterRegistry) loadCluster(ctx context.Context, schemaFilePath string) error {
	logger := log.FromContext(ctx)

	// Extract cluster name from file path
	// The file name (without directory) is used as the cluster name
	name := extractClusterName(schemaFilePath)

	logger.V(4).WithValues("cluster", name, "file", schemaFilePath).Info("Loading target cluster")

	// Create or update cluster
	cl, err := cluster.New(name, schemaFilePath, cluster.ClusterConfig{
		DevelopmentDisableAuth: cr.config.DevelopmentDisableAuth,
		GraphQLPretty:          cr.config.GraphQLPretty,
		GraphQLPlayground:      cr.config.GraphQLPlayground,
		GraphQLGraphiQL:        cr.config.GraphQLGraphiQL,
	})
	if err != nil {
		return fmt.Errorf("failed to create target cluster %s: %w", name, err)
	}

	// Store cluster
	cr.clusters[name] = cl

	return nil
}

// UpdateCluster updates an existing cluster from a schema file
func (cr *ClusterRegistry) UpdateCluster(ctx context.Context, schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	// For simplified implementation, just reload the cluster
	// TODO: if loading fails we gonna loose the existing cluster, improve this. This is atomic.
	err := cr.removeCluster(ctx, schemaFilePath)
	if err != nil {
		return err
	}

	return cr.loadCluster(ctx, schemaFilePath)
}

// RemoveCluster removes a cluster by schema file path
func (cr *ClusterRegistry) RemoveCluster(ctx context.Context, schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	return cr.removeCluster(ctx, schemaFilePath)
}

// removeCluster removes a cluster by schema file path
func (cr *ClusterRegistry) removeCluster(ctx context.Context, schemaFilePath string) error {
	logger := log.FromContext(ctx)

	// Extract cluster name from file path
	name := extractClusterName(schemaFilePath)

	logger.V(4).WithValues(
		"cluster", name,
		"file", schemaFilePath,
	).Info("Removing target cluster")

	_, exists := cr.clusters[name]
	if !exists {
		logger.V(2).WithValues(
			"cluster", name,
		).Info("Attempted to remove non-existent cluster")
		return nil
	}

	// Remove cluster (no cleanup needed in simplified version)
	delete(cr.clusters, name)

	logger.V(4).WithValues(
		"cluster", name,
	).Info("Successfully removed target cluster")

	return nil
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

// extractClusterName extracts the cluster name from a file path.
// The file name (last component of the path) is used as the cluster name.
// For example: "_output/schemas/root:bob" -> "root:bob"
func extractClusterName(filePath string) string {
	// Find the last path separator
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		// No slash found, use the whole path as the name
		return filePath
	}
	return filePath[lastSlash+1:]
}
