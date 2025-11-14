package workspacefile

import (
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

// Handler is a small function-based I/O helper. It eliminates the need for a custom interface
// while still allowing tests to inject custom behaviors via function fields.
// It intentionally mirrors the semantics of io.Reader/io.Writer style operations with
// explicit methods that delegate to the provided function fields.
//
// Zero value is not usable; prefer constructing via NewIOHandler or by explicitly setting
// all function fields.
type Handler struct {
	// ReadFunc returns the content for the given key/path.
	ReadFunc func(clusterName string) ([]byte, error)
	// WriteFunc persists the given bytes under the key/path.
	WriteFunc func(JSON []byte, clusterName string) error
	// DeleteFunc removes the content under the key/path.
	DeleteFunc func(clusterName string) error
	// schemasDir is used by the default funcs created by NewIOHandler.
	schemasDir string
}

// NewIOHandler constructs a function-based Handler that stores files under schemasDir.
// Kept name-compatible for minimal changes at call sites.
func NewIOHandler(schemasDir string) (*Handler, error) {
	if err := os.MkdirAll(schemasDir, os.ModePerm); err != nil {
		return nil, errors.Join(ErrCreateSchemasDir, err)
	}

	h := &Handler{schemasDir: schemasDir}
	// Wire default funcs using the provided directory.
	h.ReadFunc = func(clusterName string) ([]byte, error) {
		fileName := path.Join(h.schemasDir, clusterName)
		JSON, err := os.ReadFile(fileName)
		if err != nil {
			return nil, errors.Join(ErrReadJSONFile, err)
		}
		return JSON, nil
	}

	h.WriteFunc = func(JSON []byte, clusterName string) error {
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

	h.DeleteFunc = func(clusterName string) error {
		fileName := path.Join(h.schemasDir, clusterName)
		if err := os.Remove(fileName); err != nil {
			return errors.Join(ErrDeleteJSONFile, err)
		}
		return nil
	}

	return h, nil
}

// Read delegates to ReadFunc.
func (h *Handler) Read(clusterName string) ([]byte, error) {
	return h.ReadFunc(clusterName)
}

// Write delegates to WriteFunc.
func (h *Handler) Write(JSON []byte, clusterName string) error {
	return h.WriteFunc(JSON, clusterName)
}

// Delete delegates to DeleteFunc.
func (h *Handler) Delete(clusterName string) error {
	return h.DeleteFunc(clusterName)
}

// HandlerFrom wraps any value that has compatible methods (Read, Write, Delete)
// into a function-based Handler. This is primarily useful in tests where a mock
// with those methods already exists.
func HandlerFrom(h interface {
	Read(string) ([]byte, error)
	Write([]byte, string) error
	Delete(string) error
}) *Handler {
	return &Handler{
		ReadFunc:   h.Read,
		WriteFunc:  h.Write,
		DeleteFunc: h.Delete,
	}
}
