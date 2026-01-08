package clusteraccess

import (
	"context"

	"github.com/platform-mesh/golang-commons/logger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/auth"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func injectClusterMetadata(ctx context.Context, schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	// Create metadata injection config
	config := auth.MetadataInjectionConfig{
		Host: clusterAccess.Spec.Host,
		Auth: clusterAccess.Spec.Auth,
		CA:   clusterAccess.Spec.CA,
	}

	// Use the common metadata injection function
	return auth.InjectClusterMetadata(ctx, schemaJSON, config, k8sClient, log)
}
