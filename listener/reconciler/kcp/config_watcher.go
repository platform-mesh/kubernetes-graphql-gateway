package kcp

import (
	"context"
	"fmt"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/watcher"
)

// VirtualWorkspaceConfigManager interface for loading virtual workspace configurations
type VirtualWorkspaceConfigManager interface {
	LoadConfig(configPath string) (*VirtualWorkspacesConfig, error)
}

// ConfigWatcher watches the virtual workspaces configuration file for changes
type ConfigWatcher struct {
	fileWatcher      *watcher.FileWatcher
	virtualWSManager VirtualWorkspaceConfigManager
	log              *logger.Logger
	changeHandler    func(*VirtualWorkspacesConfig)
}

// NewConfigWatcher creates a new config file watcher
func NewConfigWatcher(virtualWSManager VirtualWorkspaceConfigManager, log *logger.Logger) (*ConfigWatcher, error) {
	c := &ConfigWatcher{
		virtualWSManager: virtualWSManager,
		log:              log,
	}

	fileWatcher, err := watcher.NewFileWatcher(c, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	c.fileWatcher = fileWatcher
	return c, nil
}

// Watch starts watching the configuration file and blocks until context is cancelled
func (c *ConfigWatcher) Watch(ctx context.Context, configPath string, changeHandler func(*VirtualWorkspacesConfig)) error {
	// Store change handler for use in event callbacks
	c.changeHandler = changeHandler

	// Load initial configuration
	if configPath != "" {
		if err := c.loadAndNotify(configPath); err != nil {
			c.log.Error().Err(err).Msg("failed to load initial virtual workspaces config")
		}
	}

	// Watch optional configuration file with 500ms debouncing
	return c.fileWatcher.WatchOptionalFile(ctx, configPath, 500)
}

// OnFileChanged implements watcher.FileEventHandler
func (c *ConfigWatcher) OnFileChanged(filepath string) {
	if err := c.loadAndNotify(filepath); err != nil {
		c.log.Error().Err(err).Msg("failed to reload virtual workspaces config")
	}
}

// OnFileDeleted implements watcher.FileEventHandler
func (c *ConfigWatcher) OnFileDeleted(filepath string) {
	c.log.Warn().Str("configPath", filepath).Msg("virtual workspaces config file deleted")
}

// loadAndNotify loads the config and notifies the change handler
func (c *ConfigWatcher) loadAndNotify(configPath string) error {
	config, err := c.virtualWSManager.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	c.log.Info().Int("virtualWorkspaces", len(config.VirtualWorkspaces)).Msg("loaded virtual workspaces config")

	if c.changeHandler != nil {
		c.changeHandler(config)
	}
	return nil
}
