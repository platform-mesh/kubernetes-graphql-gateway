package manager

import (
	"net/http"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/targetcluster"
)

// ClusterManager manages target clusters and their lifecycle
type ClusterManager interface {
	LoadCluster(schemaFilePath string) error
	UpdateCluster(schemaFilePath string) error
	RemoveCluster(schemaFilePath string) error
	GetCluster(name string) (*targetcluster.TargetCluster, bool)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Close() error
}

// SchemaWatcher monitors schema files and manages cluster connections
type SchemaWatcher interface {
	Initialize(watchPath string) error
	Close() error
}
