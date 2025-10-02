package kcp

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/platform-mesh/golang-commons/logger"
)

// Exported functions for testing private functions

// Cluster path exports
var ConfigForKCPClusterExported = ConfigForKCPCluster

func NewClusterPathResolverExported(cfg *rest.Config, scheme interface{}, log *logger.Logger) (*ClusterPathResolverProvider, error) {
	s, ok := scheme.(*runtime.Scheme)
	if !ok {
		return nil, fmt.Errorf("expected *runtime.Scheme, got %T", scheme)
	}
	return NewClusterPathResolver(cfg, s, log)
}

func PathForClusterFromConfigExported(clusterName string, cfg *rest.Config) (string, error) {
	return PathForClusterFromConfig(clusterName, cfg)
}

// Discovery factory exports
func NewDiscoveryFactoryExported(cfg *rest.Config) (*DiscoveryFactoryProvider, error) {
	return NewDiscoveryFactory(cfg)
}

// Error exports
var (
	ErrNilConfigExported                 = ErrNilConfig
	ErrNilSchemeExported                 = ErrNilScheme
	ErrGetClusterConfigExported          = ErrGetClusterConfig
	ErrGetLogicalClusterExported         = ErrGetLogicalCluster
	ErrMissingPathAnnotationExported     = ErrMissingPathAnnotation
	ErrParseHostURLExported              = ErrParseHostURL
	ErrClusterIsDeletedExported          = ErrClusterIsDeleted
	ErrNilDiscoveryConfigExported        = ErrNilDiscoveryConfig
	ErrGetDiscoveryClusterConfigExported = ErrGetDiscoveryClusterConfig
	ErrParseDiscoveryHostURLExported     = ErrParseDiscoveryHostURL
	ErrCreateHTTPClientExported          = ErrCreateHTTPClient
	ErrCreateDynamicMapperExported       = ErrCreateDynamicMapper
)

// Type exports
type ExportedClusterPathResolver = ClusterPathResolver
type ExportedClusterPathResolverProvider = ClusterPathResolverProvider
type ExportedDiscoveryFactory = DiscoveryFactory
type ExportedDiscoveryFactoryProvider = DiscoveryFactoryProvider

type ExportedKCPManager struct {
	*KCPManager
}

// Export private methods for testing
func (e *ExportedKCPManager) ResolveWorkspacePath(ctx context.Context, clusterName string, clusterClient client.Client) (string, error) {
	return e.KCPManager.resolveWorkspacePath(ctx, clusterName, clusterClient)
}

func (e *ExportedKCPManager) GenerateAndWriteSchemaForWorkspace(ctx context.Context, workspacePath, clusterName string) error {
	return e.KCPManager.generateAndWriteSchemaForWorkspace(ctx, workspacePath, clusterName)
}

func (e *ExportedKCPManager) CreateProviderRunnableForTesting(log *logger.Logger) ProviderRunnableInterface {
	return &providerRunnable{
		provider: e.KCPManager.provider,
		mcMgr:    e.KCPManager.mcMgr,
		log:      log,
	}
}

func (e *ExportedKCPManager) ReconcileAPIBinding(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return e.KCPManager.reconcileAPIBinding(ctx, req)
}

// Interface for testing provider runnable
type ProviderRunnableInterface interface {
	Start(ctx context.Context) error
}

// Helper function exports
var StripAPIExportPathExported = stripAPIExportPath
var ExtractAPIExportRefExported = extractAPIExportRef

// Helper function to create ClusterPathResolverProvider with custom clientFactory for testing
func NewClusterPathResolverProviderWithFactory(cfg *rest.Config, scheme *runtime.Scheme, log *logger.Logger, factory func(config *rest.Config, options client.Options) (client.Client, error)) *ClusterPathResolverProvider {
	return &ClusterPathResolverProvider{
		Scheme:        scheme,
		Config:        cfg,
		clientFactory: factory,
		log:           log,
	}
}
