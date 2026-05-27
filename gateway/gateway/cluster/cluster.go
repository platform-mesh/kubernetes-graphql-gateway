package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/roundtripper/union"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Cluster struct {
	name           string
	client         client.WithWatch
	restCfg        *rest.Config
	adminCfg       *rest.Config
	tokenReviewCfg *rest.Config
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

	cluster.adminCfg = rest.CopyConfig(cluster.restCfg)
	cluster.tokenReviewCfg = rest.CopyConfig(cluster.restCfg)
	// When TokenReviewHost is set, all TokenReviews go to that fixed host (typically
	// the gateway home workspace). Leave it empty so the request path template can
	// target /clusters/<clusterName> and TokenReview runs in the workspace being queried.
	if metadata.TokenReviewHost != "" {
		cluster.tokenReviewCfg.Host = metadata.TokenReviewHost
	}

	basePath := hostPath(metadata.Host)
	tpl := metadata.RequestPathTemplate

	cluster.adminCfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return roundtripper.NewPathTemplateHandler(rt, tpl, basePath)
	})
	if metadata.TokenReviewHost == "" {
		cluster.tokenReviewCfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return roundtripper.NewPathTemplateHandler(rt, tpl, basePath)
		})
	}

	tlsConfig := cluster.restCfg.TLSClientConfig
	baseRT, err := roundtripper.NewBaseRoundTripper(tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base roundtripper: %w", err)
	}

	dataPlanePrefix := basePath + tpl
	cluster.restCfg.Wrap(func(adminRT http.RoundTripper) http.RoundTripper {
		return union.New(
			roundtripper.NewDiscoveryHandler(roundtripper.NewPathTemplateHandler(adminRT, dataPlanePrefix, basePath)),
			roundtripper.NewBearerHandler(roundtripper.NewPathTemplateHandler(baseRT, dataPlanePrefix, basePath), roundtripper.NewUnauthorizedRoundTripper()),
		)
	})

	var mapper meta.RESTMapper
	if metadata.IntrospectionPath != "" {
		mapper, err = restMapperFromConfig(cluster.adminCfg, metadata.IntrospectionPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create REST mapper: %w", err)
		}
	}

	cluster.client, err = client.NewWithWatch(cluster.restCfg, client.Options{Mapper: mapper})
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

// RestConfig returns a copy of the cluster's rest.Config with the full
// roundtripper chain preserved, suitable for building typed clientsets.
func (c *Cluster) RestConfig() *rest.Config {
	return rest.CopyConfig(c.restCfg)
}

// TokenReviewConfig returns a rest.Config configured for validating bearer
// tokens against the workspace targeted by this cluster.
func (c *Cluster) TokenReviewConfig() *rest.Config {
	return rest.CopyConfig(c.tokenReviewCfg)
}

func (c *Cluster) Close() {
	c.client = nil
	c.adminCfg = nil
	c.restCfg = nil
	c.tokenReviewCfg = nil
}

func hostPath(host string) string {
	u, err := url.Parse(host)
	if err != nil {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}

func restMapperFromConfig(cfg *rest.Config, introspectionPath string) (meta.RESTMapper, error) {
	discoveryCfg := rest.CopyConfig(cfg)
	discoveryCfg.Host += introspectionPath

	httpClient, err := rest.HTTPClientFor(discoveryCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for discovery: %w", err)
	}
	return apiutil.NewDynamicRESTMapper(discoveryCfg, httpClient)
}
