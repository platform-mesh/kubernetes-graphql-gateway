package discoveryclient

import (
	"errors"
	"fmt"
	"net/url"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	ErrNilConfig           = errors.New("config cannot be nil")
	ErrGetClusterConfig    = errors.New("failed to get rest config for cluster")
	ErrParseHostURL        = errors.New("failed to parse rest config's Host URL")
	ErrCreateHTTPClient    = errors.New("failed to create http client")
	ErrCreateDynamicMapper = errors.New("failed to create dynamic REST mapper")
)

type Factory interface {
	ClientForCluster(name string) (discovery.DiscoveryInterface, error)
	RestMapperForCluster(name string) (meta.RESTMapper, error)
}

type NewDiscoveryIFFunc func(cfg *rest.Config) (discovery.DiscoveryInterface, error)

func discoveryCltFactory(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(cfg)
}

type FactoryProvider struct {
	*rest.Config
	NewDiscoveryIFFunc
}

func NewFactory(cfg *rest.Config) (*FactoryProvider, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	return &FactoryProvider{
		Config:             cfg,
		NewDiscoveryIFFunc: discoveryCltFactory,
	}, nil
}

func (f *FactoryProvider) ClientForCluster(name string) (discovery.DiscoveryInterface, error) {
	clusterCfg, err := configForCluster(name, f.Config)
	if err != nil {
		return nil, errors.Join(ErrGetClusterConfig, err)
	}
	return f.NewDiscoveryIFFunc(clusterCfg)
}

func (f *FactoryProvider) RestMapperForCluster(name string) (meta.RESTMapper, error) {
	clusterCfg, err := configForCluster(name, f.Config)
	if err != nil {
		return nil, errors.Join(ErrGetClusterConfig, err)
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

func configForCluster(name string, cfg *rest.Config) (*rest.Config, error) {
	clusterCfg := rest.CopyConfig(cfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, errors.Join(ErrParseHostURL, err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return clusterCfg, nil
}
