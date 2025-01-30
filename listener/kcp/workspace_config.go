package kcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func virtualWorkspaceConfigFromCfg(cfg *rest.Config, scheme *runtime.Scheme) (*rest.Config, error) {
	cfgURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config Host: %w", err)
	}
	clt, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client from config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tenancyAPIExport := &kcpapis.APIExport{}
	if err := clt.Get(ctx, client.ObjectKey{
		Name: kcptenancy.SchemeGroupVersion.Group,
	}, tenancyAPIExport); err != nil {
		return nil, fmt.Errorf("failed to get tenancy APIExport: %w", err)
	}
	virtualWorkspaces := tenancyAPIExport.Status.VirtualWorkspaces // nolint: staticcheck
	if len(virtualWorkspaces) == 0 {
		err := errors.New("empty virtual workspace list")
		return nil, fmt.Errorf("failed to get at least one virtual workspace: %w", err)
	}
	vwCFGURL, err := url.Parse(virtualWorkspaces[0].URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse virtual workspace config URL: %w", err)
	}
	cfgURL.Path = vwCFGURL.Path
	virtualWorkspaceCfg := rest.CopyConfig(cfg)
	virtualWorkspaceCfg.Host = cfgURL.String()
	return virtualWorkspaceCfg, nil
}
