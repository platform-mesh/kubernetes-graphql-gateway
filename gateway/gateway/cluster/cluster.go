package cluster

import (
	"context"
	"fmt"
	"net/http"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper/union"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Cluster struct {
	name     string
	client   client.WithWatch
	restCfg  *rest.Config
	adminCfg *rest.Config
}

// New creates a new Cluster connection from cluster metadata.
func New(
	ctx context.Context,
	name string,
	metadata *v1alpha1.ClusterMetadata,
) (*Cluster, error) {
	if metadata == nil {
		return nil, fmt.Errorf("cluster %s requires cluster metadata", name)
	}

	cluster := &Cluster{
		name: name,
	}

	var err error
	cluster.restCfg, err = v1alpha1.BuildRestConfigFromMetadata(*metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from metadata: %w", err)
	}

	// Save a copy before Wrap() mutates the transport — callers that need
	// admin-credential access (e.g. TokenReview) use this config.
	cluster.adminCfg = rest.CopyConfig(cluster.restCfg)

	tlsConfig := cluster.restCfg.TLSClientConfig
	baseRT, err := roundtripper.NewBaseRoundTripper(tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base roundtripper: %w", err)
	}

	cluster.restCfg.Wrap(func(adminRT http.RoundTripper) http.RoundTripper {
		return union.New(
			roundtripper.NewDiscoveryHandler(adminRT),
			roundtripper.NewBearerHandler(baseRT, roundtripper.NewUnauthorizedRoundTripper()),
		)
	})

	cluster.client, err = client.NewWithWatch(cluster.restCfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster client: %w", err)
	}

	logger := log.FromContext(ctx)
	logger.V(4).Info("Connected to cluster", "cluster", name)

	return cluster, nil
}

func (c *Cluster) Client() client.WithWatch {
	return c.client
}

// AdminConfig returns a rest.Config with the cluster's admin credentials,
// suitable for privileged API calls like TokenReview.
func (c *Cluster) AdminConfig() *rest.Config {
	return rest.CopyConfig(c.adminCfg)
}

func (c *Cluster) Close() {
	c.client = nil
	c.adminCfg = nil
	c.restCfg = nil
}
