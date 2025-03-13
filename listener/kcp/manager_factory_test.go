package kcp

import (
	"testing"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewManager(t *testing.T) {

	tests := map[string]struct {
		cfg          *rest.Config
		isKCPEnabled bool
		expectErr    bool
	}{
		"successful_KCP_manager_creation":          {cfg: &rest.Config{Host: validAPIServerHost}, isKCPEnabled: true, expectErr: false},
		"error_from_virtualWorkspaceConfigFromCfg": {cfg: &rest.Config{Host: schemelessAPIServerHost}, isKCPEnabled: true, expectErr: true},
		"error_from_NewClusterAwareManager":        {cfg: &rest.Config{}, isKCPEnabled: true, expectErr: true},
		"successful_manager_creation":              {cfg: &rest.Config{Host: validAPIServerHost}, isKCPEnabled: false, expectErr: false},
	}

	for name, tc := range tests {
		scheme := runtime.NewScheme()
		err := kcpapis.AddToScheme(scheme)
		assert.NoError(t, err)
		t.Run(name, func(t *testing.T) {

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects([]client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{Name: tenancyAPIExportName},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: validAPIServerHost},
						},
					},
				},
			}...).Build()
			f := &ManagerFactory{
				IsKCPEnabled: tc.isKCPEnabled,
			}
			mgr, err := f.NewManager(tc.cfg, ctrl.Options{
				Scheme: scheme,
			}, fakeClient)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, mgr)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, mgr)
		})
	}
}
