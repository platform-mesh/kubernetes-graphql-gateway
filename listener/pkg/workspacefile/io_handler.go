package workspacefile

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
)

var (
	ErrCreateSchemasDir = errors.New("failed to create or access schemas directory")
	ErrReadJSONFile     = errors.New("failed to read JSON file")
	ErrWriteJSONFile    = errors.New("failed to write JSON to file")
	ErrDeleteJSONFile   = errors.New("failed to delete JSON file")
)

// FileHandler is a simple, concrete file store for schemas.
// It provides basic Read/Write/Delete operations backed by the local filesystem.
type FileHandler struct {
	// schemasDir is the base directory where schema files are stored.
	schemasDir string
}

// NewIOHandler constructs a concrete FileHandler that stores files under schemasDir.
func NewIOHandler(schemasDir string) (*FileHandler, error) {
	if err := os.MkdirAll(schemasDir, os.ModePerm); err != nil {
		return nil, errors.Join(ErrCreateSchemasDir, err)
	}
	return &FileHandler{schemasDir: schemasDir}, nil
}

// Read reads the schema file for the given cluster name (relative path) from the schemasDir.
func (h *FileHandler) Read(_ context.Context, clusterName string) ([]byte, error) {
	fileName := path.Join(h.schemasDir, clusterName)
	JSON, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.Join(ErrReadJSONFile, err)
	}
	return JSON, nil
}

// Write writes the given JSON bytes under the clusterName path, creating subdirectories as needed.
func (h *FileHandler) Write(_ context.Context, JSON []byte, clusterName string) error {
	fileName := path.Join(h.schemasDir, clusterName)
	// Create intermediate directories if they don't exist
	dir := filepath.Dir(fileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Join(ErrWriteJSONFile, err)
	}
	if err := os.WriteFile(fileName, JSON, os.ModePerm); err != nil {
		return errors.Join(ErrWriteJSONFile, err)
	}
	return nil
}

// Delete removes the schema file for the given cluster name.
func (h *FileHandler) Delete(_ context.Context, clusterName string) error {
	fileName := path.Join(h.schemasDir, clusterName)
	if err := os.Remove(fileName); err != nil {
		return errors.Join(ErrDeleteJSONFile, err)
	}
	return nil
}
