package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/openmfp/golang-commons/logger"
)

// FileEventHandler handles file system events
type FileEventHandler interface {
	OnFileChanged(filepath string)
	OnFileDeleted(filepath string)
}

// FileWatcher provides common file watching functionality
type FileWatcher struct {
	watcher *fsnotify.Watcher
	handler FileEventHandler
	log     *logger.Logger
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(handler FileEventHandler, log *logger.Logger) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &FileWatcher{
		watcher: watcher,
		handler: handler,
		log:     log,
	}, nil
}

// WatchSingleFile watches a single file with debouncing
func (w *FileWatcher) WatchSingleFile(ctx context.Context, filePath string, debounceMs int) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Watch the directory containing the file
	fileDir := filepath.Dir(filePath)
	if err := w.watcher.Add(fileDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", fileDir, err)
	}
	defer w.watcher.Close()

	w.log.Info().Str("filePath", filePath).Msg("started watching file")

	return w.watchWithDebounce(ctx, filePath, time.Duration(debounceMs)*time.Millisecond)
}

// WatchOptionalFile watches a single file with debouncing, or waits forever if no file path is provided
// This is useful for optional configuration files where the watcher should still run even if no file is configured
func (w *FileWatcher) WatchOptionalFile(ctx context.Context, filePath string, debounceMs int) error {
	if filePath == "" {
		w.log.Info().Msg("no file path provided, waiting for graceful termination")
		<-ctx.Done()
		return nil // Graceful termination is not an error
	}

	return w.WatchSingleFile(ctx, filePath, debounceMs)
}

// WatchDirectory watches a directory recursively without debouncing
func (w *FileWatcher) WatchDirectory(ctx context.Context, dirPath string) error {
	// Add directory and subdirectories recursively
	if err := w.addWatchRecursively(dirPath); err != nil {
		return fmt.Errorf("failed to add watch paths: %w", err)
	}
	defer w.watcher.Close()

	w.log.Info().Str("dirPath", dirPath).Msg("started watching directory")

	return w.watchImmediate(ctx)
}

// watchWithDebounce handles events with debouncing for single file watching
func (w *FileWatcher) watchWithDebounce(ctx context.Context, targetFile string, debounceDelay time.Duration) error {
	var debounceTimer *time.Timer

	// Ensure timer is always stopped on function exit
	defer func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("stopping file watcher gracefully")
			return nil // Graceful termination is not an error
		case event, ok := <-w.watcher.Events:
			if !ok {
				return fmt.Errorf("file watcher events channel closed")
			}

			if w.isTargetFileEvent(event, targetFile) {
				w.log.Debug().Str("event", event.String()).Msg("file changed")

				// Simple debouncing: cancel previous timer and start new one
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					w.handler.OnFileChanged(targetFile)
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return fmt.Errorf("file watcher errors channel closed")
			}
			w.log.Error().Err(err).Msg("file watcher error")
		}
	}
}

// watchImmediate handles events immediately for directory watching
func (w *FileWatcher) watchImmediate(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("stopping directory watcher gracefully")
			return nil // Graceful termination is not an error

		case event, ok := <-w.watcher.Events:
			if !ok {
				return fmt.Errorf("directory watcher events channel closed")
			}

			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return fmt.Errorf("directory watcher errors channel closed")
			}
			w.log.Error().Err(err).Msg("directory watcher error")
		}
	}
}

// isTargetFileEvent checks if the event is for our target file
func (w *FileWatcher) isTargetFileEvent(event fsnotify.Event, targetFile string) bool {
	return filepath.Clean(event.Name) == filepath.Clean(targetFile) &&
		event.Op&(fsnotify.Write|fsnotify.Create) != 0
}

// handleEvent processes file system events for directory watching
func (w *FileWatcher) handleEvent(event fsnotify.Event) {
	w.log.Debug().Str("event", event.String()).Msg("directory event")

	filePath := event.Name
	switch event.Op {
	case fsnotify.Create, fsnotify.Write:
		// Check if this is actually a file (not a directory)
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			w.handler.OnFileChanged(filePath)
		}
		if err == nil && info.IsDir() {
			err := w.watcher.Add(filePath)
			if err != nil {
				w.log.Error().Err(err).Str("path", filePath).Msg("failed to add directory to watcher")
				return
			}
			err = filepath.WalkDir(filePath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Skip directories
				if d.IsDir() {
					return nil
				}
				w.handler.OnFileChanged(path)

				return nil
			})
			if err != nil {
				w.log.Error().Err(err).Str("path", filePath).Msg("failed to walk directory")
				return
			}
		}

	case fsnotify.Rename, fsnotify.Remove:
		w.handler.OnFileDeleted(filePath)
	default:
		w.log.Debug().Str("filepath", filePath).Str("op", event.Op.String()).Msg("unhandled file event")
	}
}

// addWatchRecursively adds the directory and all subdirectories to the watcher
func (w *FileWatcher) addWatchRecursively(dir string) error {
	if err := w.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to add watch path %s: %w", dir, err)
	}

	// Find subdirectories
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return fmt.Errorf("failed to glob directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if dirInfo, err := os.Stat(entry); err == nil && dirInfo.IsDir() {
			if err := w.addWatchRecursively(entry); err != nil {
				return err
			}
		}
	}

	return nil
}
