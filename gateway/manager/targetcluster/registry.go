package targetcluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/platform-mesh/golang-commons/logger"
	appConfig "github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// kcpWorkspaceKey is the context key for storing KCP workspace information
const kcpWorkspaceKey contextKey = "kcpWorkspace"

// ClusterRegistry manages multiple target clusters and handles HTTP routing to them
type ClusterRegistry struct {
	mu       sync.RWMutex
	clusters map[string]*TargetCluster
	log      *logger.Logger
	appCfg   appConfig.Config
}

// NewClusterRegistry creates a new cluster registry
func NewClusterRegistry(
	log *logger.Logger,
	appCfg appConfig.Config,
) *ClusterRegistry {
	return &ClusterRegistry{
		clusters: make(map[string]*TargetCluster),
		log:      log,
		appCfg:   appCfg,
	}
}

// LoadCluster loads a target cluster from a schema file
func (cr *ClusterRegistry) LoadCluster(schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Extract cluster name from file path, preserving subdirectory structure
	name := cr.extractClusterNameFromPath(schemaFilePath)

	cr.log.Info().
		Str("cluster", name).
		Str("file", schemaFilePath).
		Msg("Loading target cluster")

	// Create or update cluster
	cluster, err := NewTargetCluster(name, schemaFilePath, cr.log, cr.appCfg)
	if err != nil {
		return fmt.Errorf("failed to create target cluster %s: %w", name, err)
	}

	// Store cluster
	cr.clusters[name] = cluster

	return nil
}

// UpdateCluster updates an existing cluster from a schema file
func (cr *ClusterRegistry) UpdateCluster(schemaFilePath string) error {
	// For simplified implementation, just reload the cluster
	err := cr.RemoveCluster(schemaFilePath)
	if err != nil {
		return err
	}

	return cr.LoadCluster(schemaFilePath)
}

// RemoveCluster removes a cluster by schema file path
func (cr *ClusterRegistry) RemoveCluster(schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Extract cluster name from file path, preserving subdirectory structure
	name := cr.extractClusterNameFromPath(schemaFilePath)

	cr.log.Info().
		Str("cluster", name).
		Str("file", schemaFilePath).
		Msg("Removing target cluster")

	_, exists := cr.clusters[name]
	if !exists {
		cr.log.Warn().
			Str("cluster", name).
			Msg("Attempted to remove non-existent cluster")
		return nil
	}

	// Remove cluster (no cleanup needed in simplified version)
	delete(cr.clusters, name)

	cr.log.Info().
		Str("cluster", name).
		Msg("Successfully removed target cluster")

	return nil
}

// GetCluster returns a cluster by name
func (cr *ClusterRegistry) GetCluster(name string) (*TargetCluster, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	cluster, exists := cr.clusters[name]
	return cluster, exists
}

// Close closes all clusters and cleans up the registry
func (cr *ClusterRegistry) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for name := range cr.clusters {
		cr.log.Info().Str("cluster", name).Msg("Closed cluster during registry shutdown")
	}

	cr.clusters = make(map[string]*TargetCluster)
	cr.log.Info().Msg("Closed cluster registry")
	return nil
}

// ServeHTTP routes HTTP requests to the appropriate target cluster
func (cr *ClusterRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CORS
	if cr.handleCORS(w, r) {
		return
	}

	// Extract cluster name from path
	clusterName, r, ok := cr.extractClusterName(w, r)
	if !ok {
		return
	}

	// Get target cluster
	cluster, exists := cr.GetCluster(clusterName)
	if !exists {
		cr.log.Error().
			Str("cluster", clusterName).
			Str("path", r.URL.Path).
			Msg("Target cluster not found")
		http.NotFound(w, r)
		return
	}

	// No health checking in simplified version - clusters are either working or not loaded

	// Handle GET requests (GraphiQL/Playground) directly
	if r.Method == http.MethodGet {
		cluster.ServeHTTP(w, r)
		return
	}

	// Extract and validate token for non-GET requests
	token := GetToken(r)
	if !cr.handleAuth(w, r, token, cluster) {
		return
	}

	// Set contexts for KCP and authentication
	r = SetContexts(r, clusterName, token, cr.appCfg.EnableKcp)

	// Handle subscription requests
	if r.Header.Get("Accept") == "text/event-stream" {
		// Subscriptions will be handled by the cluster's ServeHTTP method
		cluster.ServeHTTP(w, r)
		return
	}

	// Route to target cluster
	cr.log.Debug().
		Str("cluster", clusterName).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Routing request to target cluster")

	cluster.ServeHTTP(w, r)
}

// handleAuth handles authentication for non-GET requests
func (cr *ClusterRegistry) handleAuth(w http.ResponseWriter, r *http.Request, token string, cluster *TargetCluster) bool {
	if cr.appCfg.LocalDevelopment {
		return true
	}

	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return false
	}

	if cr.appCfg.Gateway.IntrospectionAuthentication && IsIntrospectionQuery(r) {
		valid, err := cr.validateToken(r.Context(), token, cluster)
		if err != nil {
			cr.log.Error().Err(err).Str("cluster", cluster.name).Msg("Error validating token")
			http.Error(w, "Token validation failed", http.StatusUnauthorized)
			return false
		}
		if !valid {
			cr.log.Debug().Str("cluster", cluster.name).Msg("Invalid token for introspection query")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return false
		}
	}

	return true
}

// handleCORS handles CORS preflight requests and headers
func (cr *ClusterRegistry) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	if cr.appCfg.Gateway.Cors.Enabled {
		w.Header().Set("Access-Control-Allow-Origin", cr.appCfg.Gateway.Cors.AllowedOrigins)
		w.Header().Set("Access-Control-Allow-Headers", cr.appCfg.Gateway.Cors.AllowedHeaders)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return true
		}
	}
	return false
}

func (cr *ClusterRegistry) validateToken(ctx context.Context, token string, cluster *TargetCluster) (bool, error) {
	if token == "" {
		return false, errors.New("empty token")
	}

	cr.log.Debug().Str("cluster", cluster.name).Msg("Validating token for introspection query")

	valid, err := cluster.ValidateToken(ctx, token)
	if err != nil {
		cr.log.Error().Err(err).Str("cluster", cluster.name).Msg("Token validation request failed")
		return false, err
	}

	if !valid {
		cr.log.Debug().Str("cluster", cluster.name).Msg("Token validation failed - unauthorized")
		return false, nil
	}

	cr.log.Debug().Str("cluster", cluster.name).Msg("Token validation successful")
	return true, nil
}

// extractClusterName extracts the cluster name from the request path using pattern matching
// Expected formats:
//   - Regular workspace: /{clusterName}/graphql
//   - Virtual workspace: /virtual-workspace/{virtualWorkspaceName}/{kcpWorkspace}/graphql
func (cr *ClusterRegistry) extractClusterName(w http.ResponseWriter, r *http.Request) (string, *http.Request, bool) {
	clusterName, kcpWorkspace, valid := MatchURL(r.URL.Path, cr.appCfg)

	if !valid {
		cr.log.Error().
			Str("path", r.URL.Path).
			Msg(fmt.Sprintf(
				"Invalid path format, expected /{clusterName}/%s or /%s/{virtualWorkspaceName}/{kcpWorkspace}/%s",
				cr.appCfg.Url.GraphqlSuffix,
				cr.appCfg.Url.VirtualWorkspacePrefix,
				cr.appCfg.Url.GraphqlSuffix,
			))
		http.NotFound(w, r)
		return "", r, false
	}

	// Store the KCP workspace name in the request context if present
	if kcpWorkspace != "" {
		r = r.WithContext(context.WithValue(r.Context(), kcpWorkspaceKey, kcpWorkspace))
	}

	return clusterName, r, true
}

// extractClusterNameFromPath extracts cluster name from schema file path, preserving subdirectory structure
func (cr *ClusterRegistry) extractClusterNameFromPath(schemaFilePath string) string {
	// First try to find relative path from definitions directory
	if strings.Contains(schemaFilePath, "definitions/") {
		parts := strings.Split(schemaFilePath, "definitions/")
		if len(parts) >= 2 {
			relativePath := parts[len(parts)-1]
			// Remove file extension
			return strings.TrimSuffix(relativePath, filepath.Ext(relativePath))
		}
	}

	// Fallback to just filename without extension
	return strings.TrimSuffix(filepath.Base(schemaFilePath), filepath.Ext(schemaFilePath))
}
