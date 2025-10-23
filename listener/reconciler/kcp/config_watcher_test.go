package kcp

import (
	"context"
	"errors"
	"testing"

	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockVirtualWorkspaceConfigManager for testing
type MockVirtualWorkspaceConfigManager struct {
	LoadConfigFunc func(configPath string) (*VirtualWorkspacesConfig, error)
}

func (m *MockVirtualWorkspaceConfigManager) LoadConfig(configPath string) (*VirtualWorkspacesConfig, error) {
	if m.LoadConfigFunc != nil {
		return m.LoadConfigFunc(configPath)
	}
	return &VirtualWorkspacesConfig{}, nil
}

// MockVirtualWorkspaceReconciler for testing
type MockVirtualWorkspaceReconciler struct {
	ReconcileConfigFunc func(ctx context.Context, config *VirtualWorkspacesConfig) error
}

func (m *MockVirtualWorkspaceReconciler) ReconcileConfig(ctx context.Context, config *VirtualWorkspacesConfig) error {
	if m.ReconcileConfigFunc != nil {
		return m.ReconcileConfigFunc(ctx, config)
	}
	return nil
}

func TestNewConfigWatcher(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
	}{
		{
			name:        "successful_creation",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := &MockVirtualWorkspaceConfigManager{}
			reconciler := &MockVirtualWorkspaceReconciler{}

			watcher, err := NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
				assert.Equal(t, virtualWSManager, watcher.virtualWSManager)
				assert.Equal(t, reconciler, watcher.reconciler)
				assert.Equal(t, testlogger.New().Logger, watcher.log)
				assert.NotNil(t, watcher.fileWatcher)
			}
		})
	}
}

func TestConfigWatcher_OnFileChanged(t *testing.T) {
	tests := []struct {
		name                 string
		filepath             string
		loadConfigFunc       func(configPath string) (*VirtualWorkspacesConfig, error)
		expectReconcilerCall bool
	}{
		{
			name:     "successful_file_change",
			filepath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return &VirtualWorkspacesConfig{
					VirtualWorkspaces: []VirtualWorkspace{
						{Name: "test-ws", URL: "https://example.com"},
					},
				}, nil
			},
			expectReconcilerCall: true,
		},
		{
			name:     "failed_config_load",
			filepath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return nil, errors.New("failed to load config")
			},
			expectReconcilerCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := &MockVirtualWorkspaceConfigManager{
				LoadConfigFunc: tt.loadConfigFunc,
			}

			var reconcilerCalled bool
			var receivedConfig *VirtualWorkspacesConfig
			reconciler := &MockVirtualWorkspaceReconciler{
				ReconcileConfigFunc: func(ctx context.Context, config *VirtualWorkspacesConfig) error {
					reconcilerCalled = true
					receivedConfig = config
					return nil
				},
			}

			watcher, err := NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
			require.NoError(t, err)
			watcher.ctx = context.Background()

			watcher.OnFileChanged(tt.filepath)

			if tt.expectReconcilerCall {
				assert.True(t, reconcilerCalled)
				assert.NotNil(t, receivedConfig)
				assert.Equal(t, 1, len(receivedConfig.VirtualWorkspaces))
				assert.Equal(t, "test-ws", receivedConfig.VirtualWorkspaces[0].Name)
			} else {
				assert.False(t, reconcilerCalled)
			}
		})
	}
}

func TestConfigWatcher_OnFileDeleted(t *testing.T) {
	virtualWSManager := &MockVirtualWorkspaceConfigManager{}
	reconciler := &MockVirtualWorkspaceReconciler{}

	watcher, err := NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
	require.NoError(t, err)

	watcher.OnFileDeleted("/test/config.yaml")
}

func TestConfigWatcher_LoadAndNotify(t *testing.T) {
	tests := []struct {
		name                 string
		configPath           string
		loadConfigFunc       func(configPath string) (*VirtualWorkspacesConfig, error)
		expectError          bool
		expectReconcilerCall bool
	}{
		{
			name:       "successful_load_and_notify",
			configPath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return &VirtualWorkspacesConfig{
					VirtualWorkspaces: []VirtualWorkspace{
						{Name: "ws1", URL: "https://example.com"},
						{Name: "ws2", URL: "https://example.org"},
					},
				}, nil
			},
			expectError:          false,
			expectReconcilerCall: true,
		},
		{
			name:       "failed_config_load",
			configPath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return nil, errors.New("config load error")
			},
			expectError:          true,
			expectReconcilerCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := &MockVirtualWorkspaceConfigManager{
				LoadConfigFunc: tt.loadConfigFunc,
			}

			var reconcilerCalled bool
			var receivedConfig *VirtualWorkspacesConfig
			reconciler := &MockVirtualWorkspaceReconciler{
				ReconcileConfigFunc: func(ctx context.Context, config *VirtualWorkspacesConfig) error {
					reconcilerCalled = true
					receivedConfig = config
					return nil
				},
			}

			watcher, err := NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
			require.NoError(t, err)
			watcher.ctx = context.Background()

			err = watcher.loadAndNotify(tt.configPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectReconcilerCall {
				assert.True(t, reconcilerCalled)
				assert.NotNil(t, receivedConfig)
				if tt.name == "successful_load_and_notify" {
					assert.Equal(t, 2, len(receivedConfig.VirtualWorkspaces))
				}
			} else {
				assert.False(t, reconcilerCalled)
			}
		})
	}
}

func TestConfigWatcher_Watch_EmptyPath(t *testing.T) {
	virtualWSManager := &MockVirtualWorkspaceConfigManager{
		LoadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
			return &VirtualWorkspacesConfig{}, nil
		},
	}

	var reconcilerCalled bool
	reconciler := &MockVirtualWorkspaceReconciler{
		ReconcileConfigFunc: func(ctx context.Context, config *VirtualWorkspacesConfig) error {
			reconcilerCalled = true
			return nil
		},
	}

	watcher, err := NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), common.ShortTimeout)
	defer cancel()

	err = watcher.Watch(ctx, "")

	assert.NoError(t, err)
	assert.False(t, reconcilerCalled)
}
