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

var (
	errNilConfig = errors.New("config should not be nil")
	errNilScheme = errors.New("scheme should not be nil")
)

type clientFactory func(config *rest.Config, options client.Options) (client.Client, error)

type Resolver struct {
	*runtime.Scheme
	*rest.Config
	clientFactory
}

func NewResolver(cfg *rest.Config, scheme *runtime.Scheme) (*Resolver, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	if scheme == nil {
		return nil, errNilScheme
	}
	return &Resolver{
		Scheme:        scheme,
		Config:        cfg,
		clientFactory: client.New,
	}, nil
}

func (rf *Resolver) ClientForCluster(name string) (client.Client, error) {
	clusterConfig, err := getClusterConfig(name, rf.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config: %w", err)
	}
	return rf.clientFactory(clusterConfig, client.Options{Scheme: rf.Scheme})
}

func PathForCluster(name string, clt client.Client) (string, error) {
	if name == "root" {
		return name, nil
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

func getClusterConfig(name string, cfg *rest.Config) (*rest.Config, error) {
	if cfg == nil {
		return nil, errors.New("config should not be nil")
	}
	clusterCfg := rest.CopyConfig(cfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rest config's Host URL: %w", err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return clusterCfg, nil
}
