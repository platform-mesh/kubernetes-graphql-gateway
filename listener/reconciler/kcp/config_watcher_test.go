package kcp

import (
	"context"
	"errors"
	"testing"

	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/openmfp/kubernetes-graphql-gateway/common"
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

			watcher, err := NewConfigWatcher(virtualWSManager, testlogger.New().Logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
				assert.Equal(t, virtualWSManager, watcher.virtualWSManager)
				assert.Equal(t, testlogger.New().Logger, watcher.log)
				assert.NotNil(t, watcher.fileWatcher)
			}
		})
	}
}

func TestConfigWatcher_OnFileChanged(t *testing.T) {
	tests := []struct {
		name              string
		filepath          string
		loadConfigFunc    func(configPath string) (*VirtualWorkspacesConfig, error)
		expectHandlerCall bool
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
			expectHandlerCall: true,
		},
		{
			name:     "failed_config_load",
			filepath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return nil, errors.New("failed to load config")
			},
			expectHandlerCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := &MockVirtualWorkspaceConfigManager{
				LoadConfigFunc: tt.loadConfigFunc,
			}

			watcher, err := NewConfigWatcher(virtualWSManager, testlogger.New().Logger)
			require.NoError(t, err)

			// Track change handler calls
			var handlerCalled bool
			var receivedConfig *VirtualWorkspacesConfig
			changeHandler := func(config *VirtualWorkspacesConfig) {
				handlerCalled = true
				receivedConfig = config
			}
			watcher.changeHandler = changeHandler

			watcher.OnFileChanged(tt.filepath)

			if tt.expectHandlerCall {
				assert.True(t, handlerCalled)
				assert.NotNil(t, receivedConfig)
				assert.Equal(t, 1, len(receivedConfig.VirtualWorkspaces))
				assert.Equal(t, "test-ws", receivedConfig.VirtualWorkspaces[0].Name)
			} else {
				assert.False(t, handlerCalled)
			}
		})
	}
}

func TestConfigWatcher_OnFileDeleted(t *testing.T) {
	virtualWSManager := &MockVirtualWorkspaceConfigManager{}

	watcher, err := NewConfigWatcher(virtualWSManager, testlogger.New().Logger)
	require.NoError(t, err)

	// Should not panic or error
	watcher.OnFileDeleted("/test/config.yaml")
}

func TestConfigWatcher_LoadAndNotify(t *testing.T) {
	tests := []struct {
		name           string
		configPath     string
		loadConfigFunc func(configPath string) (*VirtualWorkspacesConfig, error)
		expectError    bool
		expectCall     bool
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
			expectError: false,
			expectCall:  true,
		},
		{
			name:       "failed_config_load",
			configPath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return nil, errors.New("config load error")
			},
			expectError: true,
			expectCall:  false,
		},
		{
			name:       "no_change_handler",
			configPath: "/test/config.yaml",
			loadConfigFunc: func(configPath string) (*VirtualWorkspacesConfig, error) {
				return &VirtualWorkspacesConfig{}, nil
			},
			expectError: false,
			expectCall:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualWSManager := &MockVirtualWorkspaceConfigManager{
				LoadConfigFunc: tt.loadConfigFunc,
			}

			watcher, err := NewConfigWatcher(virtualWSManager, testlogger.New().Logger)
			require.NoError(t, err)

			// Track change handler calls
			var handlerCalled bool
			var receivedConfig *VirtualWorkspacesConfig
			if tt.name != "no_change_handler" {
				changeHandler := func(config *VirtualWorkspacesConfig) {
					handlerCalled = true
					receivedConfig = config
				}
				watcher.changeHandler = changeHandler
			}

			err = watcher.loadAndNotify(tt.configPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectCall {
				assert.True(t, handlerCalled)
				assert.NotNil(t, receivedConfig)
				if tt.name == "successful_load_and_notify" {
					assert.Equal(t, 2, len(receivedConfig.VirtualWorkspaces))
				}
			} else {
				assert.False(t, handlerCalled)
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

	watcher, err := NewConfigWatcher(virtualWSManager, testlogger.New().Logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), common.ShortTimeout)
	defer cancel()

	var handlerCalled bool
	changeHandler := func(config *VirtualWorkspacesConfig) {
		handlerCalled = true
	}

	// Test with empty config path - should not try to load initial config
	err = watcher.Watch(ctx, "", changeHandler)

	// Should complete gracefully without error since graceful termination is not an error
	assert.NoError(t, err)
	assert.False(t, handlerCalled) // Should not call handler for empty path initial load
}
