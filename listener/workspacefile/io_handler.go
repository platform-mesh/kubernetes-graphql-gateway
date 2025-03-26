package workspacefile

import (
	"errors"
	"os"
	"path"
)

var (
	ErrCreateSchemasDir = errors.New("failed to create or access schemas directory")
	ErrReadJSONFile     = errors.New("failed to read JSON file")
	ErrWriteJSONFile    = errors.New("failed to write JSON to file")
)

type IOHandler struct {
	schemasDir string
}

func NewIOHandler(schemasDir string) (*IOHandler, error) {
	if err := os.MkdirAll(schemasDir, os.ModePerm); err != nil {
		return nil, errors.Join(ErrCreateSchemasDir, err)
	}

	return &IOHandler{
		schemasDir: schemasDir,
	}, nil
}

func (h *IOHandler) Read(clusterName string) ([]byte, error) {
	fileName := path.Join(h.schemasDir, clusterName)
	JSON, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.Join(ErrReadJSONFile, err)
	}
	return JSON, nil
}

func (h *IOHandler) Write(JSON []byte, clusterName string) error {
	fileName := path.Join(h.schemasDir, clusterName)
	if err := os.WriteFile(fileName, JSON, os.ModePerm); err != nil {
		return errors.Join(ErrWriteJSONFile, err)
	}
	return nil
}
