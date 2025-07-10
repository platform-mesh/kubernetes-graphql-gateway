package watcher

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/golang-commons/sentry"
)

var (
	ErrUnknownFileEvent = errors.New("unknown file event")
)

// FileEventHandler handles file system events
type FileEventHandler interface {
	OnFileChanged(filename string)
	OnFileDeleted(filename string)
}

// ClusterRegistryInterface defines the minimal interface needed from ClusterRegistry
type ClusterRegistryInterface interface {
	LoadCluster(schemaFilePath string) error
	UpdateCluster(schemaFilePath string) error
	RemoveCluster(schemaFilePath string) error
}

// FileWatcher handles file watching and delegates to cluster registry
type FileWatcher struct {
	log             *logger.Logger
	watcher         *fsnotify.Watcher
	clusterRegistry ClusterRegistryInterface
	watchPath       string
}

// NewFileWatcher creates a new watcher service
func NewFileWatcher(
	log *logger.Logger,
	clusterRegistry ClusterRegistryInterface,
) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &FileWatcher{
		log:             log,
		watcher:         watcher,
		clusterRegistry: clusterRegistry,
	}, nil
}

// Initialize sets up the watcher with the given path and processes existing files
func (s *FileWatcher) Initialize(watchPath string) error {
	s.watchPath = watchPath

	// Add path to watcher
	if err := s.watcher.Add(watchPath); err != nil {
		return fmt.Errorf("failed to add watch path: %w", err)
	}

	// Process existing files
	files, err := filepath.Glob(filepath.Join(watchPath, "*"))
	if err != nil {
		return fmt.Errorf("failed to glob files: %w", err)
	}

	for _, file := range files {
		// Load cluster directly using full path
		if err := s.clusterRegistry.LoadCluster(file); err != nil {
			s.log.Error().Err(err).Str("file", file).Msg("Failed to load cluster from existing file")
			continue
		}
	}

	// Start watching for file system events
	go s.startWatching()

	return nil
}

// startWatching begins watching for file system events (called from Initialize)
func (s *FileWatcher) startWatching() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handleEvent(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.log.Error().Err(err).Msg("Error watching files")
			sentry.CaptureError(err, nil)
		}
	}
}

// Close closes the file watcher
func (s *FileWatcher) Close() error {
	return s.watcher.Close()
}

func (s *FileWatcher) handleEvent(event fsnotify.Event) {
	s.log.Info().Str("event", event.String()).Msg("File event")

	filename := filepath.Base(event.Name)
	switch event.Op {
	case fsnotify.Create:
		s.OnFileChanged(filename)
	case fsnotify.Write:
		s.OnFileChanged(filename)
	case fsnotify.Rename:
		s.OnFileDeleted(filename)
	case fsnotify.Remove:
		s.OnFileDeleted(filename)
	default:
		err := ErrUnknownFileEvent
		s.log.Error().Err(err).Str("filename", filename).Msg("Unknown file event")
		sentry.CaptureError(sentry.SentryError(err), nil, sentry.Extras{"filename": filename, "event": event.String()})
	}
}

func (s *FileWatcher) OnFileChanged(filename string) {
	// Construct full file path
	filePath := filepath.Join(s.watchPath, filename)

	// Delegate to cluster registry
	if err := s.clusterRegistry.UpdateCluster(filePath); err != nil {
		s.log.Error().Err(err).Str("filename", filename).Str("path", filePath).Msg("Failed to update cluster")
		sentry.CaptureError(err, sentry.Tags{"filename": filename})
		return
	}

	s.log.Info().Str("filename", filename).Msg("Successfully updated cluster from file change")
}

func (s *FileWatcher) OnFileDeleted(filename string) {
	// Construct full file path
	filePath := filepath.Join(s.watchPath, filename)

	// Delegate to cluster registry
	if err := s.clusterRegistry.RemoveCluster(filePath); err != nil {
		s.log.Error().Err(err).Str("filename", filename).Str("path", filePath).Msg("Failed to remove cluster")
		sentry.CaptureError(err, sentry.Tags{"filename": filename})
		return
	}

	s.log.Info().Str("filename", filename).Msg("Successfully removed cluster from file deletion")
}
