package watcher

import (
	"github.com/openmfp/golang-commons/logger/testlogger"
)

// MockClusterRegistry is a test implementation of ClusterRegistryInterface
type MockClusterRegistry struct {
	clusters map[string]bool
}

func NewMockClusterRegistry() *MockClusterRegistry {
	return &MockClusterRegistry{
		clusters: make(map[string]bool),
	}
}

func (m *MockClusterRegistry) LoadCluster(schemaFilePath string) error {
	m.clusters[schemaFilePath] = true
	return nil
}

func (m *MockClusterRegistry) UpdateCluster(schemaFilePath string) error {
	m.clusters[schemaFilePath] = true
	return nil
}

func (m *MockClusterRegistry) RemoveCluster(schemaFilePath string) error {
	delete(m.clusters, schemaFilePath)
	return nil
}

func (m *MockClusterRegistry) HasCluster(schemaFilePath string) bool {
	_, exists := m.clusters[schemaFilePath]
	return exists
}

// NewFileWatcherForTest creates a FileWatcher instance for testing
func NewFileWatcherForTest() (*FileWatcher, error) {
	log := testlogger.New().HideLogOutput().Logger
	mockRegistry := NewMockClusterRegistry()

	return NewFileWatcher(log, mockRegistry)
}
