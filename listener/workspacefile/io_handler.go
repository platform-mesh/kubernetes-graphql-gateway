package workspacefile

import (
	"fmt"
	"os"
	"path"
)

type IOHandler struct {
	schemasDir string
}

func NewIOHandler(schemasDir string) (*IOHandler, error) {
	if err := os.MkdirAll(schemasDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create or access schemas directory: %w", err)
	}

	return &IOHandler{
		schemasDir: schemasDir,
	}, nil
}

func (h *IOHandler) Read(clusterName string) ([]byte, error) {
	fileName := path.Join(h.schemasDir, clusterName)
	JSON, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}
	return JSON, nil
}

func (h *IOHandler) Write(JSON []byte, clusterName string) error {
	fileName := path.Join(h.schemasDir, clusterName)
	if err := os.WriteFile(fileName, JSON, os.ModePerm); err != nil {
		return fmt.Errorf("failed to write JSON to file: %w", err)
	}
	return nil
}
