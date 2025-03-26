package kcp

import (
	"context"
	"errors"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
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
			err: errors.Join(ErrFailedToGetAPIExport, errors.New("apiexports.apis.kcp.io \"kubernetes.graphql.gateway\" not found")),
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			appCfg, err := config.NewFromEnv()
			assert.NoError(t, err)

			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.clientObjects != nil {
				fakeClientBuilder.WithObjects(tc.clientObjects(&appCfg)...)
			}
			fakeClient := fakeClientBuilder.Build()

			resultCfg, err := virtualWorkspaceConfigFromCfg(context.Background(), appCfg, &rest.Config{}, fakeClient)

			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				assert.Nil(t, resultCfg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.clientObjects(&appCfg)[0].(*kcpapis.APIExport).Status.VirtualWorkspaces[0].URL, resultCfg.Host) // nolint: staticcheck
			}
		})
	}
}
