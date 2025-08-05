package clusteraccess

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
)

// Metadata injector exports - now all delegated to common auth package
func InjectClusterMetadata(ctx context.Context, schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	return injectClusterMetadata(ctx, schemaJSON, clusterAccess, k8sClient, log)
}
