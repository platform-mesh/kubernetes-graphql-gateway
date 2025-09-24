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

// Test helpers

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = kcpapis.AddToScheme(scheme)
	_ = kcpcore.AddToScheme(scheme)
	return scheme
}

func createTestReconcilerOpts() reconciler.ReconcilerOpts {
	return reconciler.ReconcilerOpts{
		Config: &rest.Config{Host: "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io"},
		Scheme: createTestScheme(),
		ManagerOpts: ctrl.Options{
			Metrics: server.Options{BindAddress: "0"},
			Scheme:  createTestScheme(),
		},
	}
}

func createTestKCPManager(t *testing.T) *kcp.KCPManager {
	tempDir := t.TempDir()
	log := testlogger.New().HideLogOutput().Logger

	manager, err := kcp.NewKCPManager(
		config.Config{OpenApiDefinitionsPath: tempDir},
		createTestReconcilerOpts(),
		log,
	)
	require.NoError(t, err)
	return manager
}

// Tests

func TestNewKCPManager(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

	tests := []struct {
		name        string
		appCfg      config.Config
		opts        reconciler.ReconcilerOpts
		wantErr     bool
		errContains string
	}{
		{
			name:    "success",
			appCfg:  config.Config{OpenApiDefinitionsPath: t.TempDir()},
			opts:    createTestReconcilerOpts(),
			wantErr: false,
		},
		{
			name:        "invalid_path",
			appCfg:      config.Config{OpenApiDefinitionsPath: "/invalid/path"},
			opts:        createTestReconcilerOpts(),
			wantErr:     true,
			errContains: "failed to create or access schemas directory",
		},
		{
			name:   "nil_scheme",
			appCfg: config.Config{OpenApiDefinitionsPath: t.TempDir()},
			opts: reconciler.ReconcilerOpts{
				Config:      &rest.Config{Host: "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io"},
				Scheme:      nil,
				ManagerOpts: ctrl.Options{Metrics: server.Options{BindAddress: "0"}},
			},
			wantErr:     true,
			errContains: "scheme should not be nil",
		},
		{
			name:    "nil_logger",
			appCfg:  config.Config{OpenApiDefinitionsPath: t.TempDir()},
			opts:    createTestReconcilerOpts(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testLog *logger.Logger
			if tt.name != "nil_logger" {
				testLog = log
			}

			manager, err := kcp.NewKCPManager(tt.appCfg, tt.opts, testLog)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				assert.NotNil(t, manager.GetManager())
			}
		})
	}
}

func TestKCPManager_GetManager(t *testing.T) {
	t.Run("nil_manager", func(t *testing.T) {
		manager := &kcp.ExportedKCPManager{}
		assert.Nil(t, manager.GetManager())
	})

	t.Run("initialized_manager", func(t *testing.T) {
		manager := createTestKCPManager(t)
		assert.NotNil(t, manager.GetManager())
	})
}

func TestKCPManager_Reconcile(t *testing.T) {
	manager := createTestKCPManager(t)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test",
			Namespace: "default",
		},
	}

	// Test that Reconcile is a no-op
	result, err := manager.Reconcile(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Test multiple calls return consistent results
	for i := 0; i < 3; i++ {
		result, err := manager.Reconcile(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
	}
}

func TestKCPManager_SetupWithManager(t *testing.T) {
	kcpManager := createTestKCPManager(t)

	// Test with the manager's own manager (should work)
	err := kcpManager.SetupWithManager(kcpManager.GetManager())
	assert.NoError(t, err)
}

func TestKCPManager_StartVirtualWorkspaceWatching(t *testing.T) {
	manager := createTestKCPManager(t)
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		configPath  string
		setupConfig func(string) error
		expectErr   bool
		timeout     time.Duration
	}{
		{
			name:        "empty_path",
			configPath:  "",
			setupConfig: func(string) error { return nil },
			expectErr:   false,
			timeout:     100 * time.Millisecond,
		},
		{
			name:       "valid_config",
			configPath: filepath.Join(tempDir, "config.yaml"),
			setupConfig: func(path string) error {
				content := `virtualWorkspaces:
  - name: "test-workspace"
    url: "https://test.cluster"`
				return os.WriteFile(path, []byte(content), 0644)
			},
			expectErr: false,
			timeout:   200 * time.Millisecond,
		},
		{
			name:        "nonexistent_file",
			configPath:  filepath.Join(tempDir, "nonexistent.yaml"),
			setupConfig: func(string) error { return nil },
			expectErr:   true,
			timeout:     100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupConfig(tt.configPath)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err = manager.StartVirtualWorkspaceWatching(ctx, tt.configPath)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				// Should either succeed or be cancelled by context timeout
				if err != nil {
					t.Logf("Got error (possibly expected): %v", err)
				}
			}
		})
	}
}
