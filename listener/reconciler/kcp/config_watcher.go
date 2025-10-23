package kcp

import (
	"context"
	"fmt"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/watcher"
)

// VirtualWorkspaceConfigManager interface for loading virtual workspace configurations
type VirtualWorkspaceConfigManager interface {
	LoadConfig(configPath string) (*VirtualWorkspacesConfig, error)
}

// VirtualWorkspaceConfigReconciler interface for reconciling virtual workspace configurations
type VirtualWorkspaceConfigReconciler interface {
	ReconcileConfig(ctx context.Context, config *VirtualWorkspacesConfig) error
}

// ConfigWatcher watches the virtual workspaces configuration file for changes
type ConfigWatcher struct {
	fileWatcher      *watcher.FileWatcher
	virtualWSManager VirtualWorkspaceConfigManager
	reconciler       VirtualWorkspaceConfigReconciler
	log              *logger.Logger
	ctx              context.Context
}

// NewConfigWatcher creates a new config file watcher
func NewConfigWatcher(virtualWSManager VirtualWorkspaceConfigManager, reconciler VirtualWorkspaceConfigReconciler, log *logger.Logger) (*ConfigWatcher, error) {
	c := &ConfigWatcher{
		virtualWSManager: virtualWSManager,
		reconciler:       reconciler,
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
func (c *ConfigWatcher) Watch(ctx context.Context, configPath string) error {
	if configPath == "" {
		return nil
	}

	c.ctx = ctx

	if err := c.loadAndNotify(configPath); err != nil {
		c.log.Error().Err(err).Msg("failed to load initial virtual workspaces config")
		return err
	}

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

// loadAndNotify loads the config and reconciles it
func (c *ConfigWatcher) loadAndNotify(configPath string) error {
	config, err := c.virtualWSManager.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	c.log.Info().Int("virtualWorkspaces", len(config.VirtualWorkspaces)).Msg("loaded virtual workspaces config")

	if err := c.reconciler.ReconcileConfig(c.ctx, config); err != nil {
		c.log.Error().Err(err).Msg("failed to reconcile virtual workspaces config")
		return err
	}
	return nil
}
