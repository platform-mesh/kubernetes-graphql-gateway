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

type NewDiscoveryIFFunc func(cfg *rest.Config) (discovery.DiscoveryInterface, error)

func discoveryCltFactory(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(cfg)
}

type Factory struct {
	*rest.Config
	NewDiscoveryIFFunc
}

func NewFactory(cfg *rest.Config) (*Factory, error) {
	if cfg == nil {
		return nil, errors.New("config should not be nil")
	}
	return &Factory{
		Config:             cfg,
		NewDiscoveryIFFunc: discoveryCltFactory,
	}, nil
}

func (f *Factory) ClientForCluster(name string) (discovery.DiscoveryInterface, error) {
	clusterCfg, err := configForCluster(name, f.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config for cluster: %w", err)
	}
	return f.NewDiscoveryIFFunc(clusterCfg)
}

func (f *Factory) RestMapperForCluster(name string) (meta.RESTMapper, error) {
	clusterCfg, err := configForCluster(name, f.Config)
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
		return nil, fmt.Errorf("failed to parse rest config's Host URL: %w", err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return clusterCfg, nil
}
