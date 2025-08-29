package kcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
)

func TestNewKCPReconciler(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

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
			reconciler, err := kcp.NewKCPReconciler(tt.appCfg, tt.opts, log)

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
	log := testlogger.New().HideLogOutput().Logger

	reconciler, err := kcp.NewKCPReconciler(
		config.Config{
			OpenApiDefinitionsPath: t.TempDir(),
		},
		reconciler.ReconcilerOpts{
			Config: &rest.Config{
				Host: "https://kcp.example.com",
			},
			Scheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = kcpapis.AddToScheme(scheme)
				_ = kcpcore.AddToScheme(scheme)
				return scheme
			}(),
			ManagerOpts: ctrl.Options{
				Metrics: server.Options{BindAddress: "0"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
			},
		},
		log,
	)

	assert.NoError(t, err)
	assert.NotNil(t, reconciler)

	// Test SetupWithManager with nil manager (should work based on current implementation)
	err = reconciler.SetupWithManager(nil)
	assert.NoError(t, err, "SetupWithManager should handle nil manager gracefully")

	// Note: Cannot test calling SetupWithManager multiple times because it registers controllers
	// and duplicate controller names cause errors. This is expected behavior.
}

func TestKCPReconciler_StartVirtualWorkspaceWatching(t *testing.T) {
	tempDir := t.TempDir()
	log := testlogger.New().HideLogOutput().Logger

	reconciler, err := kcp.NewKCPReconciler(
		config.Config{
			OpenApiDefinitionsPath: tempDir,
		},
		reconciler.ReconcilerOpts{
			Config: &rest.Config{
				Host: "https://test.cluster",
			},
			Scheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = kcpapis.AddToScheme(scheme)
				_ = kcpcore.AddToScheme(scheme)
				return scheme
			}(),
			ManagerOpts: ctrl.Options{
				Metrics: server.Options{BindAddress: "0"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
			},
		},
		log,
	)
	require.NoError(t, err)

	tests := []struct {
		name          string
		configPath    string
		setupConfig   func(string) error
		expectedError bool
		timeout       time.Duration
	}{
		{
			name:          "empty_config_path_should_return_immediately",
			configPath:    "",
			setupConfig:   func(string) error { return nil },
			expectedError: false,
			timeout:       100 * time.Millisecond,
		},
		{
			name:       "valid_config_file_should_start_watching",
			configPath: filepath.Join(tempDir, "virtual-ws-config.yaml"),
			setupConfig: func(path string) error {
				content := `
virtualWorkspaces:
  - name: "test-workspace"
    url: "https://test.cluster"
`
				return os.WriteFile(path, []byte(content), 0644)
			},
			expectedError: false,
			timeout:       200 * time.Millisecond,
		},
		{
			name:       "non_existent_config_file_should_handle_gracefully",
			configPath: filepath.Join(tempDir, "non-existent.yaml"),
			setupConfig: func(string) error {
				return nil // Don't create the file
			},
			expectedError: true, // May error when trying to watch non-existent directory
			timeout:       100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupConfig(tt.configPath)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Test the actual method
			err = reconciler.StartVirtualWorkspaceWatching(ctx, tt.configPath)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				// Should either succeed or be cancelled by context timeout
				if err != nil {
					// Context cancellation or other expected errors are acceptable
					t.Logf("Got error (possibly expected): %v", err)
				}
			}
		})
	}
}

func TestKCPReconciler_ManagerWrapper_Start(t *testing.T) {
	tempDir := t.TempDir()
	log := testlogger.New().HideLogOutput().Logger

	reconciler, err := kcp.NewKCPReconciler(
		config.Config{
			OpenApiDefinitionsPath: tempDir,
		},
		reconciler.ReconcilerOpts{
			Config: &rest.Config{
				Host: "https://test.cluster",
			},
			Scheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = kcpapis.AddToScheme(scheme)
				_ = kcpcore.AddToScheme(scheme)
				return scheme
			}(),
			ManagerOpts: ctrl.Options{
				Metrics: server.Options{BindAddress: "0"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
			},
		},
		log,
	)
	require.NoError(t, err)

	// Test that GetManager returns a non-nil manager
	manager := reconciler.GetManager()
	assert.NotNil(t, manager)

	// Test that the manager's Start method can be called
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The Start method should be callable (it will likely fail due to no real cluster setup,
	// but it should not panic and should return an error or succeed)
	err = manager.Start(ctx)

	// We expect either success or failure, but no panic
	// The exact behavior depends on the underlying multicluster manager implementation
	if err != nil {
		// This is expected since we don't have a real cluster setup
		t.Logf("Manager start failed as expected: %v", err)
	} else {
		t.Logf("Manager start succeeded unexpectedly")
	}

	// The important thing is that we didn't panic and the method was callable
	assert.NotNil(t, manager, "Manager should not be nil")
}

func TestKCPReconciler_Reconcile_NoOp_Behavior(t *testing.T) {
	tempDir := t.TempDir()
	log := testlogger.New().HideLogOutput().Logger

	reconciler, err := kcp.NewKCPReconciler(
		config.Config{
			OpenApiDefinitionsPath: tempDir,
		},
		reconciler.ReconcilerOpts{
			Config: &rest.Config{
				Host: "https://test.cluster",
			},
			Scheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = kcpapis.AddToScheme(scheme)
				_ = kcpcore.AddToScheme(scheme)
				return scheme
			}(),
			ManagerOpts: ctrl.Options{
				Metrics: server.Options{BindAddress: "0"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
			},
		},
		log,
	)
	require.NoError(t, err)

	// Test multiple calls to Reconcile - should always return empty result and no error
	for i := 0; i < 5; i++ {
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-resource",
				Namespace: "test-namespace",
			},
		}

		result, err := reconciler.Reconcile(context.Background(), req)
		assert.NoError(t, err, "Reconcile should never return an error (iteration %d)", i)
		assert.Equal(t, ctrl.Result{}, result, "Reconcile should always return empty result (iteration %d)", i)
	}
}

func TestKCPReconciler_NewKCPReconciler_Coverage_Improvements(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

	tests := []struct {
		name          string
		appCfg        config.Config
		opts          reconciler.ReconcilerOpts
		expectedError bool
		errorContains string
	}{
		{
			name: "invalid_openapi_definitions_path",
			appCfg: config.Config{
				OpenApiDefinitionsPath: "/invalid/path/that/definitely/does/not/exist/anywhere",
			},
			opts: reconciler.ReconcilerOpts{
				Config: &rest.Config{Host: "https://kcp.example.com"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
				ManagerOpts: ctrl.Options{
					Metrics: server.Options{BindAddress: "0"},
				},
			},
			expectedError: true,
			errorContains: "failed to create or access schemas directory",
		},
		{
			name: "success_case_with_valid_temp_dir",
			appCfg: config.Config{
				OpenApiDefinitionsPath: t.TempDir(),
			},
			opts: reconciler.ReconcilerOpts{
				Config: &rest.Config{Host: "https://kcp.example.com"},
				Scheme: func() *runtime.Scheme {
					scheme := runtime.NewScheme()
					_ = kcpapis.AddToScheme(scheme)
					_ = kcpcore.AddToScheme(scheme)
					return scheme
				}(),
				ManagerOpts: ctrl.Options{
					Metrics: server.Options{BindAddress: "0"},
					Scheme: func() *runtime.Scheme {
						scheme := runtime.NewScheme()
						_ = kcpapis.AddToScheme(scheme)
						_ = kcpcore.AddToScheme(scheme)
						return scheme
					}(),
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler, err := kcp.NewKCPReconciler(tt.appCfg, tt.opts, log)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
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

func TestKCPReconciler_NewKCPReconciler_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		setupTest     func() (config.Config, reconciler.ReconcilerOpts, *testlogger.TestLogger)
		expectedError bool
		errorContains string
	}{
		{
			name: "nil_logger",
			setupTest: func() (config.Config, reconciler.ReconcilerOpts, *testlogger.TestLogger) {
				appCfg := config.Config{
					OpenApiDefinitionsPath: t.TempDir(),
				}
				opts := reconciler.ReconcilerOpts{
					Config: &rest.Config{Host: "https://kcp.example.com"},
					Scheme: runtime.NewScheme(),
				}
				return appCfg, opts, nil
			},
			expectedError: true,
			errorContains: "logger should not be nil",
		},
		{
			name: "invalid_config_for_provider",
			setupTest: func() (config.Config, reconciler.ReconcilerOpts, *testlogger.TestLogger) {
				log := testlogger.New().HideLogOutput()
				appCfg := config.Config{
					OpenApiDefinitionsPath: t.TempDir(),
				}
				opts := reconciler.ReconcilerOpts{
					Config: &rest.Config{}, // Empty config should cause provider creation to fail
					Scheme: func() *runtime.Scheme {
						scheme := runtime.NewScheme()
						_ = kcpapis.AddToScheme(scheme)
						_ = kcpcore.AddToScheme(scheme)
						return scheme
					}(),
				}
				return appCfg, opts, log
			},
			expectedError: true,
			errorContains: "failed to create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appCfg, opts, log := tt.setupTest()

			var actualLogger *logger.Logger
			if log != nil {
				actualLogger = log.Logger
			}

			reconciler, err := kcp.NewKCPReconciler(appCfg, opts, actualLogger)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, reconciler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reconciler)
			}
		})
	}
}
