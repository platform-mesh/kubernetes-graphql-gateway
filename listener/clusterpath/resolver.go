package clusterpath

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resolver struct {
	*runtime.Scheme
	*rest.Config
	ResolverFunc
}

type ResolverFunc func(name string, cfg *rest.Config, scheme *runtime.Scheme) (string, error)

func Resolve(name string, cfg *rest.Config, scheme *runtime.Scheme) (string, error) {
	if name == "root" {
		return name, nil
	}
	if cfg == nil {
		return "", errors.New("config should not be nil")
	}
	if scheme == nil {
		return "", errors.New("scheme should not be nil")
	}
	clusterCfg := rest.CopyConfig(cfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return "", fmt.Errorf("failed to parse rest config Host URL: %w", err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	clt, err := client.New(clusterCfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create client for cluster: %w", err)
	}
	lc := &kcpcore.LogicalCluster{}
	if err := clt.Get(context.TODO(), client.ObjectKey{Name: "cluster"}, lc); err != nil {
		return "", fmt.Errorf("failed to get logicalcluster resource: %w", err)
	}
	path, ok := lc.GetAnnotations()["kcp.io/path"]
	if !ok {
		return "", errors.New("failed to get cluster path from kcp.io/path annotation")
	}
	return path, nil
}
