package clusteraccess

import (
	"errors"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
)

// BuildTargetClusterConfigFromTyped extracts connection info from ClusterAccess and builds rest.Config
func BuildTargetClusterConfigFromTyped(clusterAccess v1alpha1.ClusterAccess, k8sClient client.Client) (*rest.Config, string, error) {
	spec := clusterAccess.Spec

	// Extract host (required)
	host := spec.Host
	if host == "" {
		return nil, "", errors.New("host field not found in ClusterAccess spec")
	}

	// Extract cluster name (path field or resource name)
	clusterName := clusterAccess.GetName()
	if spec.Path != "" {
		clusterName = spec.Path
	}

	// Use common auth package to build config
	config, err := auth.BuildConfig(host, spec.Auth, spec.CA, k8sClient)
	if err != nil {
		return nil, "", err
	}

	return config, clusterName, nil
}
