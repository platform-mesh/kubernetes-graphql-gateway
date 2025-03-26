package kcp

import (
	"context"
	"errors"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

type ManagerFactory struct {
	appConfig config.Config
}

func NewManagerFactory(appCfg config.Config) *ManagerFactory {
	return &ManagerFactory{
		appConfig: appCfg,
	}
}

func (f *ManagerFactory) NewManager(ctx context.Context, restCfg *rest.Config, opts ctrl.Options, clt client.Client) (manager.Manager, error) {
	if !f.appConfig.EnableKcp {
		return ctrl.NewManager(restCfg, opts)
	}

	virtualWorkspaceCfg, err := virtualWorkspaceConfigFromCfg(ctx, f.appConfig, restCfg, clt)
	if err != nil {
		return nil, errors.Join(ErrGetVWConfig, err)
	}

	return kcpctrl.NewClusterAwareManager(virtualWorkspaceCfg, opts)
}
