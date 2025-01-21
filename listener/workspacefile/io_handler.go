package workspacefile

import (
	"fmt"
	"os"
	"path"
)

type IOHandler interface {
	Reader
	Writer
}

type IOHandlerImpl struct {
	SchemasDir string
}

func NewIOHandler(schemasDir string) (*IOHandlerImpl, error) {
	_, err := os.Stat(schemasDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(schemasDir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create openAPI definitions dir: %w", err)
		}
	}
	return &IOHandlerImpl{
		SchemasDir: schemasDir,
	}, nil
}

func (h *IOHandlerImpl) Read(clusterName string) ([]byte, error) {
	fileName := path.Join(h.SchemasDir, clusterName)
	JSON, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}
	return JSON, nil
}

func (h *IOHandlerImpl) Write(JSON []byte, clusterName string) error {
	fileName := path.Join(h.SchemasDir, clusterName)
	err := os.WriteFile(fileName, JSON, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write JSON to file: %w", err)
	}
	return nil
}
