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

type Factory interface {
	ClientForCluster(name string) (*discovery.DiscoveryClient, error)
	RestMapperForCluster(name string) (meta.RESTMapper, error)
}

type FactoryImpl struct {
	restCfg *rest.Config
}

func NewFactory(cfg *rest.Config) (*FactoryImpl, error) {
	if cfg == nil {
		return nil, errors.New("config should not be nil")
	}
	return &FactoryImpl{restCfg: cfg}, nil
}

func (f *FactoryImpl) ClientForCluster(name string) (*discovery.DiscoveryClient, error) {
	clusterCfg, err := configForCluster(name, f.restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config for cluster: %w", err)
	}
	return discovery.NewDiscoveryClientForConfig(clusterCfg)
}

func (f *FactoryImpl) RestMapperForCluster(name string) (meta.RESTMapper, error) {
	clusterCfg, err := configForCluster(name, f.restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config for cluster: %w", err)
	}
	httpClt, err := rest.HTTPClientFor(clusterCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}
	return apiutil.NewDynamicRESTMapper(clusterCfg, httpClt)
}

func configForCluster(name string, cfg *rest.Config) (*rest.Config, error) {
	clusterCfg := rest.CopyConfig(cfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rest config Host URL: %w", err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return clusterCfg, nil
}
