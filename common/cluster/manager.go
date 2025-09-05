package cluster

import (
	"context"
	"fmt"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
)

// Manager provides unified access to clusters across different runtime modes
type Manager interface {
	// GetCluster returns a cluster by name
	GetCluster(ctx context.Context, name string) (Cluster, error)

	// ListClusters returns all available clusters
	ListClusters(ctx context.Context) ([]string, error)

	// IsMulticluster returns true if this manager uses multicluster runtime
	IsMulticluster() bool
}

// MulticlusterManager wraps multicluster runtime manager
type MulticlusterManager struct {
	mcMgr mcmanager.Manager
}

// NewMulticlusterManager creates a new multicluster manager wrapper
func NewMulticlusterManager(mcMgr mcmanager.Manager) *MulticlusterManager {
	return &MulticlusterManager{
		mcMgr: mcMgr,
	}
}

func (m *MulticlusterManager) GetCluster(ctx context.Context, name string) (Cluster, error) {
	cluster, err := m.mcMgr.GetCluster(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s from multicluster manager: %w", name, err)
	}
	return NewMulticlusterRuntimeCluster(name, cluster), nil
}

func (m *MulticlusterManager) ListClusters(ctx context.Context) ([]string, error) {
	// Note: multicluster runtime doesn't provide a direct ListClusters method
	// This would need to be implemented based on the specific multicluster runtime version
	// For now, return empty list - clusters are discovered dynamically
	return []string{}, nil
}

func (m *MulticlusterManager) IsMulticluster() bool {
	return true
}

// StandaloneManager manages standalone clusters (legacy mode)
type StandaloneManager struct {
	clusters map[string]Cluster
}

// NewStandaloneManager creates a new standalone cluster manager
func NewStandaloneManager() *StandaloneManager {
	return &StandaloneManager{
		clusters: make(map[string]Cluster),
	}
}

func (m *StandaloneManager) GetCluster(ctx context.Context, name string) (Cluster, error) {
	cluster, exists := m.clusters[name]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", name)
	}
	return cluster, nil
}

func (m *StandaloneManager) ListClusters(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names, nil
}

func (m *StandaloneManager) IsMulticluster() bool {
	return false
}

// AddCluster adds a cluster to the standalone manager
func (m *StandaloneManager) AddCluster(cluster Cluster) {
	m.clusters[cluster.GetName()] = cluster
}

// RemoveCluster removes a cluster from the standalone manager
func (m *StandaloneManager) RemoveCluster(name string) {
	delete(m.clusters, name)
}
