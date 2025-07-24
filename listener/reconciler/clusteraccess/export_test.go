package clusteraccess

import (
	"context"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
)

// Exported functions for testing private functions

// Config builder exports
// ExtractCAData exposes the common auth ExtractCAData function for testing
func ExtractCAData(ctx context.Context, ca *gatewayv1alpha1.CAConfig, k8sClient client.Client) ([]byte, error) {
	return auth.ExtractCAData(ctx, ca, k8sClient)
}

// ConfigureAuthentication exposes the common auth ConfigureAuthentication function for testing
func ConfigureAuthentication(ctx context.Context, config *rest.Config, authConfig *gatewayv1alpha1.AuthConfig, k8sClient client.Client) error {
	return auth.ConfigureAuthentication(ctx, config, authConfig, k8sClient)
}

func ExtractAuthFromKubeconfig(config *rest.Config, authInfo *api.AuthInfo) error {
	return auth.ExtractAuthFromKubeconfig(config, authInfo)
}

// Metadata injector exports - now all delegated to common auth package
func InjectClusterMetadata(ctx context.Context, schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	return injectClusterMetadata(ctx, schemaJSON, clusterAccess, k8sClient, log)
}

// The following functions are now part of the common auth package
// and can be accessed directly from there for testing if needed

// Subroutines exports
type GenerateSchemaSubroutine = generateSchemaSubroutine

func NewGenerateSchemaSubroutine(reconciler *ExportedClusterAccessReconciler) *GenerateSchemaSubroutine {
	return &generateSchemaSubroutine{reconciler: reconciler}
}

// Type and constant exports
type ExportedCRDStatus = CRDStatus
type ExportedClusterAccessReconciler = ClusterAccessReconciler

const (
	ExportedCRDNotRegistered = CRDNotRegistered
	ExportedCRDRegistered    = CRDRegistered
)

// Error exports
var (
	ExportedErrCRDNotRegistered = ErrCRDNotRegistered
	ExportedErrCRDCheckFailed   = ErrCRDCheckFailed
)
