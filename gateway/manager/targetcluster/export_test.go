package targetcluster

import (
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/rest"

	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

// BuildConfigFromMetadata exposes the internal buildConfigFromMetadata function for testing
func BuildConfigFromMetadata(metadata *ClusterMetadata, log *logger.Logger) (*rest.Config, error) {
	return buildConfigFromMetadata(metadata, log)
}

// NewTestTargetCluster creates a TargetCluster with the specified name for testing
func NewTestTargetCluster(name string) *TargetCluster {
	return &TargetCluster{
		name: name,
	}
}

// CreateTestConfig creates an appConfig.Config for testing with the specified settings
func CreateTestConfig(localDev bool, gatewayPort string) appConfig.Config {
	config := appConfig.Config{
		LocalDevelopment: localDev,
	}
	config.Gateway.Port = gatewayPort
	config.Url.VirtualWorkspacePrefix = "virtual-workspace"
	config.Url.DefaultKcpWorkspace = "root"
	config.Url.GraphqlSuffix = "graphql"
	return config
}
