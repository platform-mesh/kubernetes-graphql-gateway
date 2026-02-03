package watcher

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/platform-mesh/golang-commons/sentry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/watcher"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SchemaEventHandler handles schema change events from watchers.
type SchemaEventHandler interface {
	OnSchemaChanged(ctx context.Context, clusterName string, schema string)
	OnSchemaDeleted(ctx context.Context, clusterName string)
}

// FileWatcher watches a directory for schema files and notifies the handler.
type FileWatcher struct {
	fileWatcher *watcher.FileWatcher
	handler     SchemaEventHandler
	watchPath   string
}

// NewFileWatcher creates a new file watcher that will notify the given handler
// when schema files change.
func NewFileWatcher(handler SchemaEventHandler) (*FileWatcher, error) {
	fw := &FileWatcher{
		handler: handler,
	}

	fileWatcher, err := watcher.NewFileWatcher(fw)
	if err != nil {
		return nil, err
	}

	fw.fileWatcher = fileWatcher
	return fw, nil
}

// Run starts the file watcher and blocks until the context is cancelled.
// It first loads all existing files, then watches for changes.
func (fw *FileWatcher) Run(ctx context.Context, watchPath string) error {
	logger := log.FromContext(ctx)
	fw.watchPath = watchPath

	// Process all existing files first
	if err := fw.loadAllFiles(ctx, watchPath); err != nil {
		return err
	}

	// Start watching directory (this blocks until context is cancelled)
	if err := fw.fileWatcher.WatchDirectory(ctx, watchPath); err != nil {
		logger.Error(err, "directory watcher stopped")
		return err
	}

	return nil
}

// OnFileChanged implements watcher.FileEventHandler.
// It reads the file and notifies the schema handler.
func (fw *FileWatcher) OnFileChanged(ctx context.Context, filePath string) {
	logger := log.FromContext(ctx)

	// Check if this is actually a file (not a directory)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return
	}

	// Read the file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error(err, "Failed to read schema file", "path", filePath)
		sentry.CaptureError(err, sentry.Tags{"filepath": filePath})
		return
	}

	// Extract cluster name from file path and notify handler
	clusterName := extractClusterName(filePath)
	fw.handler.OnSchemaChanged(ctx, clusterName, string(data))

	logger.Info("Successfully processed schema file change", "path", filePath, "cluster", clusterName)
}

// OnFileDeleted implements watcher.FileEventHandler.
// It notifies the schema handler that a schema was deleted.
func (fw *FileWatcher) OnFileDeleted(ctx context.Context, filePath string) {
	logger := log.FromContext(ctx)

	// Extract cluster name from file path and notify handler
	clusterName := extractClusterName(filePath)
	fw.handler.OnSchemaDeleted(ctx, clusterName)

	logger.Info("Successfully processed schema file deletion", "path", filePath, "cluster", clusterName)
}

// loadAllFiles loads all files in the directory and subdirectories
func (fw *FileWatcher) loadAllFiles(ctx context.Context, dir string) error {
	logger := log.FromContext(ctx)

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Read and process the file
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Error(err, "Failed to read schema file", "file", path)
			return nil // Continue processing other files
		}

		clusterName := extractClusterName(path)
		fw.handler.OnSchemaChanged(ctx, clusterName, string(data))

		return nil
	})
}

// extractClusterName extracts the cluster name from a file path.
// The file name (last component of the path) is used as the cluster name.
// For example: "_output/schemas/root:bob" -> "root:bob"
func extractClusterName(filePath string) string {
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return filePath
	}
	return filePath[lastSlash+1:]
}
