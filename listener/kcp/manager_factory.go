package kcp

import (
	"context"

	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

type ManagerFactory struct {
	appConfig config.Config
	log       *logger.Logger
}

func NewManagerFactory(log *logger.Logger, appCfg config.Config) *ManagerFactory {
	return &ManagerFactory{
		log:       log,
		appConfig: appCfg,
	}
}

func (f *ManagerFactory) NewManager(ctx context.Context, restCfg *rest.Config, opts ctrl.Options, clt client.Client) (manager.Manager, error) {
	if !f.appConfig.EnableKcp {
		return ctrl.NewManager(restCfg, opts)
	}

	return kcpctrl.NewClusterAwareManager(restCfg, opts)
}
