package kcp

import (
	"context"
	"errors"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVirtualWorkspaceConfigFromCfg(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, kcpapis.AddToScheme(scheme))

	tests := map[string]struct {
		clientObjects func(appCfg *config.Config) []client.Object
		err           error
	}{
		"successful_configuration_update": {
			clientObjects: func(appCfg *config.Config) []client.Object {
				return []client.Object{
					&kcpapis.APIExport{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: appCfg.ApiExportWorkspace,
							Name:      appCfg.ApiExportName,
						},
						Status: kcpapis.APIExportStatus{
							VirtualWorkspaces: []kcpapis.VirtualWorkspace{
								{URL: "https://192.168.1.13:6443/services/apiexport/root/tenancy.kcp.io"},
							},
						},
					},
				}
			},
		},
		"error_retrieving_APIExport": {
			err: errors.Join(ErrFailedToGetAPIExport, errors.New("apiexports.apis.kcp.io \"tenancy.kcp.io\" not found")),
		},
		"empty_virtual_workspace_list": {
			clientObjects: func(appCfg *config.Config) []client.Object {
				return []client.Object{
					&kcpapis.APIExport{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: appCfg.ApiExportWorkspace,
							Name:      appCfg.ApiExportName,
						},
					},
				}
			},
			err: ErrNoVirtualURLsFound,
		},
		"empty_virtual_workspace_url": {
			clientObjects: func(appCfg *config.Config) []client.Object {
				return []client.Object{
					&kcpapis.APIExport{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: appCfg.ApiExportWorkspace,
							Name:      appCfg.ApiExportName,
						},
						Status: kcpapis.APIExportStatus{
							VirtualWorkspaces: []kcpapis.VirtualWorkspace{
								{URL: ""},
							},
						},
					},
				}
			},
			err: ErrEmptyVirtualWorkspaceURL,
		},
		"wrong_url_in_virtual_ws": {
			clientObjects: func(appCfg *config.Config) []client.Object {
				return []client.Object{
					&kcpapis.APIExport{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: appCfg.ApiExportWorkspace,
							Name:      appCfg.ApiExportName,
						},
						Status: kcpapis.APIExportStatus{
							VirtualWorkspaces: []kcpapis.VirtualWorkspace{
								{URL: "ht@tp://bad_url"},
							},
						},
					},
				}
			},
			err: errors.Join(ErrInvalidURL, errors.New("parse \"ht@tp://bad_url\": first path segment in URL cannot contain colon")),
		},
	}

	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			appCfg, err := config.NewFromEnv()
			assert.NoError(t, err)
			appCfg.LocalDevelopment = true

			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.clientObjects != nil {
				fakeClientBuilder.WithObjects(tc.clientObjects(&appCfg)...)
			}
			fakeClient := fakeClientBuilder.Build()

			resultCfg, err := virtualWorkspaceConfigFromCfg(context.Background(), log, appCfg, &rest.Config{Host: validAPIServerHost}, fakeClient)

			if tc.err != nil {
				// here it fails
				assert.EqualError(t, err, tc.err.Error())
				assert.Nil(t, resultCfg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.clientObjects(&appCfg)[0].(*kcpapis.APIExport).Status.VirtualWorkspaces[0].URL, resultCfg.Host) // nolint: staticcheck
			}
		})
	}
}

func TestCombineBaseURLAndPath(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		pathURL  string
		expected string
		err      error
	}{
		{
			name:     "success",
			baseURL:  "https://openmfp-kcp-front-proxy.openmfp-system:8443/clusters/root",
			pathURL:  "https://kcp.dev.local:8443/services/apiexport/root/kubernetes.graphql.gateway",
			expected: "https://openmfp-kcp-front-proxy.openmfp-system:8443/services/apiexport/root/kubernetes.graphql.gateway",
		},
		{
			name:     "success_base_with_port",
			baseURL:  "https://example.com:8080",
			pathURL:  "/api/resource",
			expected: "https://example.com:8080/api/resource",
		},
		{
			name:     "success_base_with_subpath_relative_path",
			baseURL:  "https://example.com/base",
			pathURL:  "api/resource",
			expected: "https://example.com/api/resource",
		},
		{
			name:     "success_base_with_subpath_absolute_path",
			baseURL:  "https://example.com/base",
			pathURL:  "/api/resource",
			expected: "https://example.com/api/resource",
		},
		{
			name:     "success_empty_path_url",
			baseURL:  "https://example.com",
			pathURL:  "",
			expected: "https://example.com/",
		},
		{
			name:    "error_invalid_base_url",
			baseURL: "ht@tp://bad_url",
			pathURL: "/api/resource",
			err:     errors.Join(ErrInvalidURL, errors.New("parse \"ht@tp://bad_url\": first path segment in URL cannot contain colon")),
		},
		{
			name:    "error_invalid_path_url",
			baseURL: "https://example.com",
			pathURL: "ht@tp://bad_url",
			err:     errors.Join(ErrInvalidURL, errors.New("parse \"ht@tp://bad_url\": first path segment in URL cannot contain colon")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := combineBaseURLAndPath(tt.baseURL, tt.pathURL)

			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
