package clusteraccess

import (
	"context"
	"errors"

	"github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/auth"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildTargetClusterConfigFromTyped extracts connection info from ClusterAccess and builds rest.Config
func BuildTargetClusterConfigFromTyped(ctx context.Context, clusterAccess v1alpha1.ClusterAccess, k8sClient client.Client) (*rest.Config, string, error) {
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
	config, err := auth.BuildConfig(ctx, spec.Auth, spec.CA, k8sClient)
	if err != nil {
		return nil, "", err
	}

	// Set host
	config.Host = host

	return config, clusterName, nil
}
