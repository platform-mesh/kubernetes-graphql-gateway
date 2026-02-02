package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/platform-mesh/golang-commons/sentry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/registry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/watcher"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FileWatcher handles file watching and delegates to cluster registry
type FileWatcher struct {
	fileWatcher     *watcher.FileWatcher
	clusterRegistry *registry.ClusterRegistry
	watchPath       string
}

// NewFileWatcher creates a new watcher service
func NewFileWatcher(
	clusterRegistry *registry.ClusterRegistry,
) (*FileWatcher, error) {
	fw := &FileWatcher{
		clusterRegistry: clusterRegistry,
	}

	fileWatcher, err := watcher.NewFileWatcher(fw)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	fw.fileWatcher = fileWatcher
	return fw, nil
}

// Initialize sets up the watcher with the given context and path and processes existing files
func (s *FileWatcher) Initialize(ctx context.Context, watchPath string) error {
	logger := log.FromContext(ctx)
	s.watchPath = watchPath

	// Process all existing files first
	if err := s.loadAllFiles(ctx, watchPath); err != nil {
		return fmt.Errorf("failed to load files: %w", err)
	}

	// Start watching directory in background goroutine
	go func() {
		if err := s.fileWatcher.WatchDirectory(ctx, watchPath); err != nil {
			logger.Error(err, "directory watcher stopped")
		}
	}()

	return nil
}

// OnFileChanged implements watcher.FileEventHandler
func (s *FileWatcher) OnFileChanged(ctx context.Context, filePath string) {
	logger := log.FromContext(ctx)
	// Check if this is actually a file (not a directory)
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		return
	}

	// Delegate to cluster registry
	if err := s.clusterRegistry.UpdateCluster(ctx, filePath); err != nil {
		logger.Error(err, "Failed to update cluster", "path", filePath)
		sentry.CaptureError(err, sentry.Tags{"filepath": filePath})
		return
	}

	logger.Info("Successfully updated cluster from file change", "path", filePath)
}

// OnFileDeleted implements watcher.FileEventHandler
func (s *FileWatcher) OnFileDeleted(ctx context.Context, filePath string) {
	logger := log.FromContext(ctx)
	// Delegate to cluster registry
	if err := s.clusterRegistry.RemoveCluster(ctx, filePath); err != nil {
		logger.Error(err, "Failed to remove cluster", "path", filePath)
		sentry.CaptureError(err, sentry.Tags{"filepath": filePath})
		return
	}

	logger.Info("Successfully removed cluster from file deletion", "path", filePath)
}

// loadAllFiles loads all files in the directory and subdirectories
func (s *FileWatcher) loadAllFiles(ctx context.Context, dir string) error {
	logger := log.FromContext(ctx)
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Load cluster directly using full path
		if err := s.clusterRegistry.LoadCluster(ctx, path); err != nil {
			logger.Error(err, "Failed to load cluster from file", "file", path)
			// Continue processing other files instead of failing
		}

		return nil
	})
}
