package kcp

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	provider "github.com/kcp-dev/multicluster-provider/apiexport"
)

var _ multicluster.Provider = &Provider{}
var _ multicluster.ProviderRunnable = &Provider{}

// Provider is a [sigs.k8s.io/multicluster-runtime/pkg/multicluster.Provider] that represents each [logical cluster]
// (in the kcp sense) exposed via a APIExport virtual workspace as a cluster in the [sigs.k8s.io/multicluster-runtime] sense.
//
// Core functionality is delegated to the apiexport.Provider, this wrapper just adds best effort path-awareness index to it via hooks.
//
// [logical cluster]: https://docs.kcp.io/kcp/latest/concepts/terminology/#logical-cluster
type Provider struct {
	*provider.Provider
}

// New is simple provider wrapper for kcp.
func New(cfg *rest.Config, endpointSliceName string, options provider.Options) (*Provider, error) {
	p, err := provider.New(cfg, endpointSliceName, options)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Provider: p,
	}, nil
}

// Start starts the provider and blocks.
func (p *Provider) Start(ctx context.Context, aware multicluster.Aware) error {
	return p.Provider.Start(ctx, &awareWrapper{Aware: aware})
}

// awareWrapper wraps a multicluster.Aware to create kube-bind Cluster objects
// so we could bootstrap RBAC for them.
type awareWrapper struct {
	multicluster.Aware
}

func (a *awareWrapper) Engage(ctx context.Context, name multicluster.ClusterName, cluster cluster.Cluster) error {
	return a.Aware.Engage(ctx, name, cluster)
}
