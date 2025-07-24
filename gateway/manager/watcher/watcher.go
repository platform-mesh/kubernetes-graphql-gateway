package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/golang-commons/sentry"
	"github.com/openmfp/kubernetes-graphql-gateway/common/watcher"
)

// ClusterRegistryInterface defines the minimal interface needed from ClusterRegistry
type ClusterRegistryInterface interface {
	LoadCluster(schemaFilePath string) error
	UpdateCluster(schemaFilePath string) error
	RemoveCluster(schemaFilePath string) error
}

// FileWatcher handles file watching and delegates to cluster registry
type FileWatcher struct {
	log             *logger.Logger
	fileWatcher     *watcher.FileWatcher
	clusterRegistry ClusterRegistryInterface
	watchPath       string
}

// NewFileWatcher creates a new watcher service
func NewFileWatcher(
	log *logger.Logger,
	clusterRegistry ClusterRegistryInterface,
) (*FileWatcher, error) {
	fw := &FileWatcher{
		log:             log,
		clusterRegistry: clusterRegistry,
	}

	fileWatcher, err := watcher.NewFileWatcher(fw, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	fw.fileWatcher = fileWatcher
	return fw, nil
}

// Initialize sets up the watcher with the given context and path and processes existing files
func (s *FileWatcher) Initialize(ctx context.Context, watchPath string) error {
	s.watchPath = watchPath

	// Process all existing files first
	if err := s.loadAllFiles(watchPath); err != nil {
		return fmt.Errorf("failed to load files: %w", err)
	}

	// Start watching directory in background goroutine
	go func() {
		if err := s.fileWatcher.WatchDirectory(ctx, watchPath); err != nil {
			s.log.Error().Err(err).Msg("directory watcher stopped")
		}
	}()

	return nil
}

// OnFileChanged implements watcher.FileEventHandler
func (s *FileWatcher) OnFileChanged(filePath string) {
	// Check if this is actually a file (not a directory)
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		return
	}

	// Delegate to cluster registry
	if err := s.clusterRegistry.UpdateCluster(filePath); err != nil {
		s.log.Error().Err(err).Str("path", filePath).Msg("Failed to update cluster")
		sentry.CaptureError(err, sentry.Tags{"filepath": filePath})
		return
	}

	s.log.Info().Str("path", filePath).Msg("Successfully updated cluster from file change")
}

// OnFileDeleted implements watcher.FileEventHandler
func (s *FileWatcher) OnFileDeleted(filePath string) {
	// Delegate to cluster registry
	if err := s.clusterRegistry.RemoveCluster(filePath); err != nil {
		s.log.Error().Err(err).Str("path", filePath).Msg("Failed to remove cluster")
		sentry.CaptureError(err, sentry.Tags{"filepath": filePath})
		return
	}

	s.log.Info().Str("path", filePath).Msg("Successfully removed cluster from file deletion")
}

// loadAllFiles loads all files in the directory and subdirectories
func (s *FileWatcher) loadAllFiles(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Load cluster directly using full path
		if err := s.clusterRegistry.LoadCluster(path); err != nil {
			s.log.Error().Err(err).Str("file", path).Msg("Failed to load cluster from file")
			// Continue processing other files instead of failing
		}

		return nil
	})
}
