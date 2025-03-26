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
	ErrNilConfig             = errors.New("config cannot be nil")
	ErrNilScheme             = errors.New("scheme should not be nil")
	ErrGetClusterConfig      = errors.New("failed to get cluster config")
	ErrGetLogicalCluster     = errors.New("failed to get logicalcluster resource")
	ErrMissingPathAnnotation = errors.New("failed to get cluster path from kcp.io/path annotation")
	ErrParseHostURL          = errors.New("failed to parse rest config's Host URL")
)

type clientFactory func(config *rest.Config, options client.Options) (client.Client, error)

type Resolver struct {
	*runtime.Scheme
	*rest.Config
	clientFactory
}

func NewResolver(cfg *rest.Config, scheme *runtime.Scheme) (*Resolver, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if scheme == nil {
		return nil, ErrNilScheme
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
		return nil, errors.Join(ErrGetClusterConfig, err)
	}
	return rf.clientFactory(clusterConfig, client.Options{Scheme: rf.Scheme})
}

func PathForCluster(name string, clt client.Client) (string, error) {
	if name == "root" {
		return name, nil
	}
	lc := &kcpcore.LogicalCluster{}
	if err := clt.Get(context.TODO(), client.ObjectKey{Name: "cluster"}, lc); err != nil {
		return "", errors.Join(ErrGetLogicalCluster, err)
	}
	path, ok := lc.GetAnnotations()["kcp.io/path"]
	if !ok {
		return "", ErrMissingPathAnnotation
	}
	return path, nil
}

func getClusterConfig(name string, cfg *rest.Config) (*rest.Config, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	clusterCfg := rest.CopyConfig(cfg)
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, errors.Join(ErrParseHostURL, err)
	}
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", name)
	clusterCfg.Host = clusterCfgURL.String()
	return clusterCfg, nil
}
