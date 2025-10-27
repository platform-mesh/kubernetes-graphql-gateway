package kcp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
			virtualWSManager := mocks.NewMockVirtualWorkspaceConfigManager(t)
			reconciler := mocks.NewMockVirtualWorkspaceConfigReconciler(t)

			watcher, err := kcp.NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
			}
		})
	}
}

func TestConfigWatcher_OnFileChanged(t *testing.T) {
	tests := []struct {
		name       string
		filepath   string
		setupMocks func(*mocks.MockVirtualWorkspaceConfigManager, *mocks.MockVirtualWorkspaceConfigReconciler)
	}{
		{
			name:     "successful_file_change",
			filepath: "/test/config.yaml",
			setupMocks: func(manager *mocks.MockVirtualWorkspaceConfigManager, reconciler *mocks.MockVirtualWorkspaceConfigReconciler) {
				config := &kcp.VirtualWorkspacesConfig{
					VirtualWorkspaces: []kcp.VirtualWorkspace{
						{Name: "test-ws", URL: "https://example.com"},
					},
				}
				manager.EXPECT().LoadConfig("/test/config.yaml").Return(config, nil)
				reconciler.EXPECT().ReconcileConfig(mock.Anything, config).Return(nil)
			},
		},
		{
			name:     "failed_config_load",
			filepath: "/test/config.yaml",
			setupMocks: func(manager *mocks.MockVirtualWorkspaceConfigManager, reconciler *mocks.MockVirtualWorkspaceConfigReconciler) {
				manager.EXPECT().LoadConfig("/test/config.yaml").Return((*kcp.VirtualWorkspacesConfig)(nil), errors.New("failed to load config"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := mocks.NewMockVirtualWorkspaceConfigManager(t)
			reconciler := mocks.NewMockVirtualWorkspaceConfigReconciler(t)

			tt.setupMocks(virtualWSManager, reconciler)

			watcher, err := kcp.NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
			require.NoError(t, err)

			watcher.OnFileChanged(tt.filepath)
		})
	}
}

func TestConfigWatcher_OnFileDeleted(t *testing.T) {
	virtualWSManager := mocks.NewMockVirtualWorkspaceConfigManager(t)
	reconciler := mocks.NewMockVirtualWorkspaceConfigReconciler(t)

	watcher, err := kcp.NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
	require.NoError(t, err)

	watcher.OnFileDeleted("/test/config.yaml")
}

func TestConfigWatcher_Watch_EmptyPath(t *testing.T) {
	virtualWSManager := mocks.NewMockVirtualWorkspaceConfigManager(t)
	reconciler := mocks.NewMockVirtualWorkspaceConfigReconciler(t)

	watcher, err := kcp.NewConfigWatcher(virtualWSManager, reconciler, testlogger.New().Logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), common.ShortTimeout)
	defer cancel()

	err = watcher.Watch(ctx, "")

	assert.NoError(t, err)
}
