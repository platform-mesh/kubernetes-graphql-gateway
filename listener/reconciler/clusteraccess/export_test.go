package clusteraccess

import (
	"context"

	"github.com/platform-mesh/golang-commons/logger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Metadata injector exports - now all delegated to common auth package
func InjectClusterMetadata(ctx context.Context, schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger, defaultSAExpirationSeconds int64) ([]byte, error) {
	return injectClusterMetadata(ctx, schemaJSON, clusterAccess, k8sClient, log, defaultSAExpirationSeconds)
}
