package kcp

import (
	"fmt"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type ManagerFactory struct {
	IsKCPEnabled bool
}

func (f *ManagerFactory) NewManager(cfg *rest.Config, opts ctrl.Options, clt client.Client) (manager.Manager, error) {
	if !f.IsKCPEnabled {
		return ctrl.NewManager(cfg, opts)
	}
	virtualWorkspaceCfg, err := virtualWorkspaceConfigFromCfg(cfg, clt)
	if err != nil {
		return nil, fmt.Errorf("unable to get virtual workspace config: %w", err)
	}
	return kcpctrl.NewClusterAwareManager(virtualWorkspaceCfg, opts)
}
