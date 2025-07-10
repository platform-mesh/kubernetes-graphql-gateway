package kcp

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Exported functions for testing private functions

// Cluster path exports
var ConfigForKCPClusterExported = ConfigForKCPCluster

func NewClusterPathResolverExported(cfg *rest.Config, scheme interface{}) (*ClusterPathResolverProvider, error) {
	return NewClusterPathResolver(cfg, scheme.(*runtime.Scheme))
}

func PathForClusterExported(name string, clt client.Client) (string, error) {
	return PathForCluster(name, clt)
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
type ExportedAPIBindingReconciler = APIBindingReconciler
type ExportedKCPReconciler = KCPReconciler

// Helper function to create ClusterPathResolverProvider with custom clientFactory for testing
func NewClusterPathResolverProviderWithFactory(cfg *rest.Config, scheme *runtime.Scheme, factory func(config *rest.Config, options client.Options) (client.Client, error)) *ClusterPathResolverProvider {
	return &ClusterPathResolverProvider{
		Scheme:        scheme,
		Config:        cfg,
		clientFactory: factory,
	}
}
