package kcp_test

import (
	"testing"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp"
)

func TestNewKCPReconciler(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name        string
		appCfg      config.Config
		opts        reconciler.ReconcilerOpts
		wantErr     bool
		errContains string
	}{
		{
			name: "successful_creation",
			appCfg: config.Config{
				OpenApiDefinitionsPath: t.TempDir(),
			},
			opts: reconciler.ReconcilerOpts{
				Config: &rest.Config{
					Host: "https://kcp.example.com",
				},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					// Register KCP types
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
				ManagerOpts: ctrl.Options{
					Metrics: server.Options{BindAddress: "0"}, // Disable metrics for tests
					Scheme: func() *runtime.Scheme {
						scheme := runtime.NewScheme()
						// Register KCP types
						_ = kcpapis.AddToScheme(scheme)
						_ = kcpcore.AddToScheme(scheme)
						return scheme
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid_openapi_definitions_path",
			appCfg: config.Config{
				OpenApiDefinitionsPath: "/invalid/path/that/does/not/exist",
			},
			opts: reconciler.ReconcilerOpts{
				Config: &rest.Config{
					Host: "https://kcp.example.com",
				},
				Scheme: runtime.NewScheme(),
				ManagerOpts: ctrl.Options{
					Metrics: server.Options{BindAddress: "0"},
				},
			},
			wantErr:     true,
			errContains: "failed to create or access schemas directory",
		},
		{
			name: "nil_scheme",
			appCfg: config.Config{
				OpenApiDefinitionsPath: t.TempDir(),
			},
			opts: reconciler.ReconcilerOpts{
				Config: &rest.Config{
					Host: "https://kcp.example.com",
				},
				Scheme: nil,
				ManagerOpts: ctrl.Options{
					Metrics: server.Options{BindAddress: "0"},
				},
			},
			wantErr:     true,
			errContains: "scheme should not be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler, err := kcp.NewKCPReconciler(tt.appCfg, tt.opts, mockLogger)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, reconciler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reconciler)
				assert.NotNil(t, reconciler.GetManager())
			}
		})
	}
}

func TestKCPReconciler_GetManager(t *testing.T) {
	reconciler := &kcp.ExportedKCPReconciler{}

	// Since GetManager() just returns the manager field, we can test it simply
	assert.Nil(t, reconciler.GetManager())

	// Test with a real manager would require more setup, so we'll keep this simple
}

func TestKCPReconciler_Reconcile(t *testing.T) {
	reconciler := &kcp.ExportedKCPReconciler{}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test",
			Namespace: "default",
		},
	}

	// The Reconcile method should be a no-op and always return empty result with no error
	result, err := reconciler.Reconcile(t.Context(), req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestKCPReconciler_SetupWithManager(t *testing.T) {
	reconciler := &kcp.ExportedKCPReconciler{}

	// The SetupWithManager method should be a no-op and always return no error
	// since controllers are set up in the constructor
	err := reconciler.SetupWithManager(nil)

	assert.NoError(t, err)
}
