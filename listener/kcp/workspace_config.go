package kcp

import (
	"context"
	"errors"
	"fmt"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/openmfp/golang-commons/logger"
	"net/url"
	"strings"
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
	ErrInvalidURL               = errors.New("invalid URL format")
)

func virtualWorkspaceConfigFromCfg(
	ctx context.Context,
	log *logger.Logger,
	appCfg config.Config,
	restCfg *rest.Config,
	clt client.Client,
) (*rest.Config, error) {
	timeOutDuration := 10 * time.Second
	ctx, cancelFn := context.WithTimeout(ctx, timeOutDuration)
	defer cancelFn()

	var apiExport kcpapis.APIExport
	key := client.ObjectKey{
		Namespace: appCfg.ApiExportWorkspace,
		Name:      appCfg.ApiExportName,
	}
	if err := clt.Get(ctx, key, &apiExport); err != nil {
		// if this is not a local development, we must have kubernetes.graphql.gateway apiexport
		if !appCfg.LocalDevelopment {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errors.Join(ErrTimeoutFetchingAPIExport, err)
			}
			return nil, errors.Join(ErrFailedToGetAPIExport, err)
		}

		// otherwise fallback to the default APIExport, but live ApiBinding watching will not work
		if err = clt.Get(ctx, client.ObjectKey{Name: kcptenancy.SchemeGroupVersion.Group}, &apiExport); err != nil {
			return nil, errors.Join(ErrFailedToGetAPIExport, err)
		}

		log.Warn().Msg(fmt.Sprintf("failed to find %s ApiExport, listener will not watch ApiBinding changes in realtime", appCfg.ApiExportName))
	}

	if len(apiExport.Status.VirtualWorkspaces) == 0 { // nolint: staticcheck
		return nil, ErrNoVirtualURLsFound
	}

	virtualWorkspaceURL := apiExport.Status.VirtualWorkspaces[0].URL // nolint: staticcheck
	if virtualWorkspaceURL == "" {
		return nil, ErrEmptyVirtualWorkspaceURL
	}

	internalVirtualWorkspaceURL, err := combineBaseURLAndPath(restCfg.Host, virtualWorkspaceURL)
	if err != nil {
		return nil, err
	}

	restCfg.Host = internalVirtualWorkspaceURL

	return restCfg, nil
}

func combineBaseURLAndPath(baseURLStr, pathURLStr string) (string, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return "", errors.Join(ErrInvalidURL, err)
	}

	pathURL, err := url.Parse(pathURLStr)
	if err != nil {
		return "", errors.Join(ErrInvalidURL, err)
	}

	path := pathURL.Path

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return baseURL.ResolveReference(&url.URL{Path: path}).String(), nil
}
