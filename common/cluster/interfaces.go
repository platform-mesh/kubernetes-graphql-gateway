package cluster

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

// Cluster represents a unified interface for accessing Kubernetes clusters
// This interface works with both multicluster runtime clusters and standalone clusters
type Cluster interface {
	// GetName returns the cluster name/identifier
	GetName() string

	// GetClient returns a controller-runtime client for this cluster
	GetClient() client.Client

	// GetConfig returns the rest.Config for this cluster
	GetConfig() *rest.Config

	// GetRESTMapper returns the REST mapper for this cluster
	GetRESTMapper() meta.RESTMapper

	// GetDiscoveryClient returns a discovery client for this cluster
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
}

// MulticlusterRuntimeCluster wraps a controller-runtime cluster.Cluster
// to implement our unified Cluster interface
type MulticlusterRuntimeCluster struct {
	name    string
	cluster cluster.Cluster
}

// NewMulticlusterRuntimeCluster creates a new wrapper for multicluster runtime clusters
func NewMulticlusterRuntimeCluster(name string, cluster cluster.Cluster) *MulticlusterRuntimeCluster {
	return &MulticlusterRuntimeCluster{
		name:    name,
		cluster: cluster,
	}
}

func (c *MulticlusterRuntimeCluster) GetName() string {
	return c.name
}

func (c *MulticlusterRuntimeCluster) GetClient() client.Client {
	return c.cluster.GetClient()
}

func (c *MulticlusterRuntimeCluster) GetConfig() *rest.Config {
	return c.cluster.GetConfig()
}

func (c *MulticlusterRuntimeCluster) GetRESTMapper() meta.RESTMapper {
	return c.cluster.GetRESTMapper()
}

func (c *MulticlusterRuntimeCluster) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(c.cluster.GetConfig())
}

// StandaloneCluster represents a standalone cluster (non-multicluster runtime)
type StandaloneCluster struct {
	name      string
	client    client.Client
	config    *rest.Config
	mapper    meta.RESTMapper
	discovery discovery.DiscoveryInterface
}

// NewStandaloneCluster creates a new standalone cluster
func NewStandaloneCluster(name string, client client.Client, config *rest.Config, mapper meta.RESTMapper) (*StandaloneCluster, error) {
	discovery, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return &StandaloneCluster{
		name:      name,
		client:    client,
		config:    config,
		mapper:    mapper,
		discovery: discovery,
	}, nil
}

func (c *StandaloneCluster) GetName() string {
	return c.name
}

func (c *StandaloneCluster) GetClient() client.Client {
	return c.client
}

func (c *StandaloneCluster) GetConfig() *rest.Config {
	return c.config
}

func (c *StandaloneCluster) GetRESTMapper() meta.RESTMapper {
	return c.mapper
}

func (c *StandaloneCluster) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return c.discovery, nil
}
