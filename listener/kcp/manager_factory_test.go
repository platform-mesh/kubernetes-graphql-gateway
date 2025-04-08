package kcp

import (
	"context"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/stretchr/testify/require"
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
		isKCPEnabled bool
		expectErr    bool
	}{
		"successful_KCP_manager_creation": {isKCPEnabled: true, expectErr: false},
		"successful_manager_creation":     {isKCPEnabled: false, expectErr: false},
	}

	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	for name, tc := range tests {
		scheme := runtime.NewScheme()
		err := kcpapis.AddToScheme(scheme)
		assert.NoError(t, err)
		t.Run(name, func(t *testing.T) {
			appCfg, err := config.NewFromEnv()
			assert.NoError(t, err)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects([]client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: appCfg.ApiExportWorkspace,
						Name:      appCfg.ApiExportName,
					},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: validAPIServerHost},
						},
					},
				},
			}...).Build()

			f := NewManagerFactory(log, appCfg)

			mgr, err := f.NewManager(
				context.Background(),
				&rest.Config{Host: validAPIServerHost},
				ctrl.Options{Scheme: scheme},
				fakeClient,
			)

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
