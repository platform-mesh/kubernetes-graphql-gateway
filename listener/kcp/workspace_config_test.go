package kcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tenancyAPIExportName    = "tenancy.kcp.io"
	validAPIServerHost      = "https://192.168.1.13:6443"
	schemelessAPIServerHost = "://192.168.1.13:6443"
)

func TestVirtualWorkspaceConfigFromCfg(t *testing.T) {
	scheme := runtime.NewScheme()
	err := kcpapis.AddToScheme(scheme)
	assert.NoError(t, err)
	tests := map[string]struct {
		cfg           *rest.Config
		clientObjects []client.Object
		expectErr     bool
	}{
		"successful_configuration_update": {
			cfg: &rest.Config{Host: validAPIServerHost},
			clientObjects: []client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{Name: tenancyAPIExportName},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: "https://192.168.1.13:6443/services/apiexport/root/tenancy.kcp.io"},
						},
					},
				},
			},
			expectErr: false,
		},
		"invalid_config_host_url": {
			cfg:       &rest.Config{Host: schemelessAPIServerHost},
			expectErr: true,
		},
		"error_retrieving_APIExport": {
			cfg:       &rest.Config{Host: validAPIServerHost},
			expectErr: true,
		},
		"empty_virtual_workspace_list": {
			cfg: &rest.Config{Host: validAPIServerHost},
			clientObjects: []client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{Name: tenancyAPIExportName},
				},
			},
			expectErr: true,
		},
		"invalid_virtual_workspace_url": {
			cfg: &rest.Config{Host: validAPIServerHost},
			clientObjects: []client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{Name: tenancyAPIExportName},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: schemelessAPIServerHost},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.clientObjects...).Build()

			resultCfg, err := virtualWorkspaceConfigFromCfg(tc.cfg, fakeClient)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, resultCfg)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resultCfg)
		})
	}
}
