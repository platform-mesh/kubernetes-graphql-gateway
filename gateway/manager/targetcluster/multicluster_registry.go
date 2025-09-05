package targetcluster

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/platform-mesh/golang-commons/logger"
	commoncluster "github.com/platform-mesh/kubernetes-graphql-gateway/common/cluster"
	appConfig "github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
)

// MulticlusterClusterRegistry manages clusters using multicluster runtime
type MulticlusterClusterRegistry struct {
	mu                  sync.RWMutex
	clusters            map[string]*TargetCluster
	log                 *logger.Logger
	appCfg              appConfig.Config
	roundTripperFactory RoundTripperFactory
	clusterManager      commoncluster.Manager
}

// NewMulticlusterClusterRegistry creates a new multicluster-aware cluster registry
func NewMulticlusterClusterRegistry(
	log *logger.Logger,
	appCfg appConfig.Config,
	roundTripperFactory RoundTripperFactory,
	clusterManager commoncluster.Manager,
) *MulticlusterClusterRegistry {
	return &MulticlusterClusterRegistry{
		clusters:            make(map[string]*TargetCluster),
		log:                 log,
		appCfg:              appCfg,
		roundTripperFactory: roundTripperFactory,
		clusterManager:      clusterManager,
	}
}

// LoadCluster loads a target cluster from a schema file, but also integrates with multicluster runtime
func (cr *MulticlusterClusterRegistry) LoadCluster(schemaFilePath string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Extract cluster name from file path, preserving subdirectory structure
	name := cr.extractClusterNameFromPath(schemaFilePath)

	cr.log.Info().
		Str("cluster", name).
		Str("file", schemaFilePath).
		Msg("Loading target cluster with multicluster runtime integration")

	// Try to get cluster from multicluster runtime first
	ctx := context.Background()
	mcCluster, err := cr.clusterManager.GetCluster(ctx, name)
	if err != nil {
		cr.log.Debug().
			Err(err).
			Str("cluster", name).
			Msg("Cluster not found in multicluster runtime, using schema file")
	}

	// Create or update cluster - if we have multicluster runtime cluster, use it
	var cluster *TargetCluster
	if mcCluster != nil {
		cluster, err = NewTargetClusterFromMulticluster(name, schemaFilePath, mcCluster, cr.log, cr.appCfg, cr.roundTripperFactory)
	} else {
		cluster, err = NewTargetCluster(name, schemaFilePath, cr.log, cr.appCfg, cr.roundTripperFactory)
	}

	if err != nil {
		return fmt.Errorf("failed to create target cluster %s: %w", name, err)
	}

	// Store cluster
	cr.clusters[name] = cluster

	return nil
}

// UpdateCluster updates an existing cluster from a schema file
func (cr *MulticlusterClusterRegistry) UpdateCluster(schemaFilePath string) error {
	// For simplified implementation, just reload the cluster
	err := cr.RemoveCluster(schemaFilePath)
	if err != nil {
		return err
	}

	return cr.LoadCluster(schemaFilePath)
}

// RemoveCluster removes a cluster by schema file path
func (cr *MulticlusterClusterRegistry) RemoveCluster(schemaFilePath string) error {
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
func (cr *MulticlusterClusterRegistry) GetCluster(name string) (*TargetCluster, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	cluster, exists := cr.clusters[name]
	return cluster, exists
}

// ServeHTTP handles HTTP requests and routes them to appropriate clusters
func (cr *MulticlusterClusterRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract cluster name from URL path or headers
	clusterName := cr.extractClusterFromRequest(r)

	cr.mu.RLock()
	cluster, exists := cr.clusters[clusterName]
	cr.mu.RUnlock()

	if !exists {
		cr.log.Warn().
			Str("cluster", clusterName).
			Str("path", r.URL.Path).
			Msg("Cluster not found for request")
		http.Error(w, "Cluster not found", http.StatusNotFound)
		return
	}

	// Delegate to the target cluster
	cluster.ServeHTTP(w, r)
}

// Close closes all clusters and cleans up the registry
func (cr *MulticlusterClusterRegistry) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for name := range cr.clusters {
		cr.log.Info().Str("cluster", name).Msg("Closing cluster")
	}

	cr.clusters = make(map[string]*TargetCluster)
	return nil
}

// extractClusterNameFromPath extracts cluster name from schema file path
func (cr *MulticlusterClusterRegistry) extractClusterNameFromPath(schemaFilePath string) string {
	// Remove file extension and get the relative path from definitions directory
	name := strings.TrimSuffix(filepath.Base(schemaFilePath), filepath.Ext(schemaFilePath))

	// For subdirectories, include the directory structure
	dir := filepath.Dir(schemaFilePath)
	if dir != "." && dir != "" {
		// Get the last directory component to preserve structure like "virtual-workspace/cluster-name"
		parentDir := filepath.Base(dir)
		if parentDir != "." && parentDir != "" {
			name = parentDir + "/" + name
		}
	}

	return name
}

// extractClusterFromRequest extracts the target cluster name from the HTTP request
func (cr *MulticlusterClusterRegistry) extractClusterFromRequest(r *http.Request) string {
	// Try to get cluster from URL path first (e.g., /cluster/cluster-name/...)
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) >= 2 && pathParts[0] == "cluster" {
		return pathParts[1]
	}

	// Try to get from custom header
	if clusterName := r.Header.Get("X-Cluster-Name"); clusterName != "" {
		return clusterName
	}

	// Default to "default" cluster
	return "default"
}
