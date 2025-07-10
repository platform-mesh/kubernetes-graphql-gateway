package kcp

import (
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	ErrNilDiscoveryConfig        = errors.New("config cannot be nil")
	ErrGetDiscoveryClusterConfig = errors.New("failed to get rest config for cluster")
	ErrParseDiscoveryHostURL     = errors.New("failed to parse rest config's Host URL")
	ErrCreateHTTPClient          = errors.New("failed to create http client")
	ErrCreateDynamicMapper       = errors.New("failed to create dynamic REST mapper")
)

type DiscoveryFactory interface {
	ClientForCluster(name string) (discovery.DiscoveryInterface, error)
	RestMapperForCluster(name string) (meta.RESTMapper, error)
}

type NewDiscoveryIFFunc func(cfg *rest.Config) (discovery.DiscoveryInterface, error)

func discoveryCltFactory(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(cfg)
}

type DiscoveryFactoryProvider struct {
	*rest.Config
	NewDiscoveryIFFunc
}

func NewDiscoveryFactory(cfg *rest.Config) (*DiscoveryFactoryProvider, error) {
	if cfg == nil {
		return nil, ErrNilDiscoveryConfig
	}
	return &DiscoveryFactoryProvider{
		Config:             cfg,
		NewDiscoveryIFFunc: discoveryCltFactory,
	}, nil
}

func (f *DiscoveryFactoryProvider) ClientForCluster(name string) (discovery.DiscoveryInterface, error) {
	clusterCfg, err := ConfigForKCPCluster(name, f.Config)
	if err != nil {
		return nil, errors.Join(ErrGetDiscoveryClusterConfig, err)
	}
	return f.NewDiscoveryIFFunc(clusterCfg)
}

func (f *DiscoveryFactoryProvider) RestMapperForCluster(name string) (meta.RESTMapper, error) {
	clusterCfg, err := ConfigForKCPCluster(name, f.Config)
	if err != nil {
		return nil, errors.Join(ErrGetDiscoveryClusterConfig, err)
	}
	httpClt, err := rest.HTTPClientFor(clusterCfg)
	if err != nil {
		return nil, errors.Join(ErrCreateHTTPClient, err)
	}
	mapper, err := apiutil.NewDynamicRESTMapper(clusterCfg, httpClt)
	if err != nil {
		return nil, errors.Join(ErrCreateDynamicMapper, err)
	}
	return mapper, nil
}
