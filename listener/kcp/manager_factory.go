package kcp

import (
	"fmt"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/flags"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type NewManagerFunc func(cfg *rest.Config, opts ctrl.Options) (manager.Manager, error)

func ManagerFactory(opFlags *flags.Flags) NewManagerFunc {
	if opFlags.EnableKcp {
		return NewKcpManager
	}
	return ctrl.NewManager
}

func NewKcpManager(cfg *rest.Config, opts ctrl.Options) (manager.Manager, error) {
	virtualWorkspaceCfg, err := virtualWorkspaceConfigFromCfg(cfg, opts.Scheme)
	if err != nil {
		return nil, fmt.Errorf("unable to get virtual workspace config: %w", err)
	}
	mgr, err := kcpctrl.NewClusterAwareManager(virtualWorkspaceCfg, opts)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate manager: %w", err)
	}
	return mgr, nil
}
