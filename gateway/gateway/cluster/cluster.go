package cluster

import (
	"context"
	"fmt"
	"net/http"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper/union"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Cluster represents a connection to a Kubernetes cluster.
type Cluster struct {
	name    string
	client  client.WithWatch
	restCfg *rest.Config
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

	useSAAuth := metadata.Auth != nil && metadata.Auth.Type == v1alpha1.AuthTypeServiceAccount

	// For SA auth, create a client to the local cluster for TokenRequest API
	var localClient client.Client
	if useSAAuth {
		localClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{})
		if err != nil {
			return nil, fmt.Errorf("failed to create local cluster client for SA auth: %w", err)
		}
	}

	tlsConfig := cluster.restCfg.TLSClientConfig
	baseRT, err := roundtripper.NewBaseRoundTripper(tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base roundtripper: %w", err)
	}

	cluster.restCfg.Wrap(func(adminRT http.RoundTripper) http.RoundTripper {

		roundTripperChain := []union.Handler{
			roundtripper.NewDiscoveryHandler(adminRT),
		}

		if useSAAuth {
			roundTripperChain = append(roundTripperChain, roundtripper.NewServiceAccountHandler(baseRT, localClient, roundtripper.ServiceAccountConfig{
				Name:      metadata.Auth.SAName,
				Namespace: metadata.Auth.SANamespace,
				Audience:  metadata.Auth.SAAudience,
			}),
			)
		} else {
			roundTripperChain = append(roundTripperChain,
				roundtripper.NewBearerHandler(baseRT, roundtripper.NewUnauthorizedRoundTripper()),
			)
		}

		return union.New(roundTripperChain...)
	})

	cluster.client, err = client.NewWithWatch(cluster.restCfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster client: %w", err)
	}

	logger := log.FromContext(ctx)
	logger.V(4).Info("Connected to cluster", "cluster", name)

	return cluster, nil
}

// Client returns the Kubernetes client for this cluster.
func (c *Cluster) Client() client.WithWatch {
	return c.client
}
