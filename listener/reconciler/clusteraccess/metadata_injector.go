package clusteraccess

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
)

func injectClusterMetadata(ctx context.Context, schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	// Determine the path
	path := clusterAccess.Spec.Path
	if path == "" {
		path = clusterAccess.GetName()
	}

	// Create metadata injection config
	config := auth.MetadataInjectionConfig{
		Host: clusterAccess.Spec.Host,
		Path: path,
		Auth: clusterAccess.Spec.Auth,
		CA:   clusterAccess.Spec.CA,
	}

	// Use the common metadata injection function
	return auth.InjectClusterMetadata(ctx, schemaJSON, config, k8sClient, log)
}
