package targetcluster

import (
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/rest"
)

// BuildConfigFromMetadata exposes the internal buildConfigFromMetadata function for testing
func BuildConfigFromMetadata(metadata *ClusterMetadata, log *logger.Logger) (*rest.Config, error) {
	return buildConfigFromMetadata(metadata, log)
}
