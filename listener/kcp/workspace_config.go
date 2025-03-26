package kcp

import (
	"context"
	"errors"
	"time"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

var (
	ErrTimeoutFetchingAPIExport = errors.New("timeout fetching APIExport")
	ErrFailedToGetAPIExport     = errors.New("failed to get APIExport")
	ErrNoVirtualURLsFound       = errors.New("no virtual URLs found for APIExport")
	ErrEmptyVirtualWorkspaceURL = errors.New("empty URL in virtual workspace for APIExport")
)

func virtualWorkspaceConfigFromCfg(ctx context.Context, appCfg config.Config, restCfg *rest.Config, clt client.Client) (*rest.Config, error) {
	timeOutDuration := 10 * time.Second
	ctx, cancelFn := context.WithTimeout(ctx, timeOutDuration)
	defer cancelFn()

	var apiExport kcpapis.APIExport
	key := client.ObjectKey{
		Namespace: appCfg.ApiExportWorkspace,
		Name:      appCfg.ApiExportName,
	}
	if err := clt.Get(ctx, key, &apiExport); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.Join(ErrTimeoutFetchingAPIExport, err)
		}
		return nil, errors.Join(ErrFailedToGetAPIExport, err)
	}

	if len(apiExport.Status.VirtualWorkspaces) == 0 { // nolint: staticcheck
		return nil, ErrNoVirtualURLsFound
	}

	virtualWorkspaceURL := apiExport.Status.VirtualWorkspaces[0].URL // nolint: staticcheck
	if virtualWorkspaceURL == "" {
		return nil, ErrEmptyVirtualWorkspaceURL
	}

	restCfg.Host = virtualWorkspaceURL

	return restCfg, nil
}
