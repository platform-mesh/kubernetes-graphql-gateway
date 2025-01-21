package discoveryclient

import (
	"errors"
	"fmt"
	"net/url"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type Factory interface {
	ClientForCluster(name string) (*discovery.DiscoveryClient, error)
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

func (h *FactoryImpl) ClientForCluster(name string) (*discovery.DiscoveryClient, error) {
	clusterCfg := rest.CopyConfig(h.restCfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rest config Host URL: %w", err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return discovery.NewDiscoveryClientForConfig(clusterCfg)
}
