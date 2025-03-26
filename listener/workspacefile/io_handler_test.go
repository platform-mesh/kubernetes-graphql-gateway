package workspacefile

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testJSON = []byte("{\"key\":\"value\"}")

func TestNewIOHandler(t *testing.T) {
	tempDir := t.TempDir()

	tests := map[string]struct {
		schemasDir string
		expectErr  bool
	}{
		"valid_directory":        {schemasDir: tempDir, expectErr: false},
		"non_existent_directory": {schemasDir: path.Join(tempDir, "non-existent"), expectErr: false},
		"invalid_directory":      {schemasDir: "/dev/null/schemas", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewIOHandler(tc.schemasDir)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestRead(t *testing.T) {
	tempDir := t.TempDir()

	validClusterName := "root:sap:openmfp"

	validFile := filepath.Join(tempDir, validClusterName)

	err := os.WriteFile(validFile, testJSON, 0644)
	assert.NoError(t, err)

	handler := &IOHandler{
		schemasDir: tempDir,
	}

	tests := map[string]struct {
		clusterName string
		expectErr   bool
	}{
		"valid_file":        {clusterName: validClusterName, expectErr: false},
		"non_existent_file": {clusterName: "root:non-existent", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := handler.Read(tc.clusterName)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWrite(t *testing.T) {
	tempDir := t.TempDir()
	handler := &IOHandler{
		schemasDir: tempDir,
	}

	tests := map[string]struct {
		clusterName string
		expectErr   bool
	}{
		"valid_write":  {clusterName: "root:sap:openmfp", expectErr: false},
		"invalid_path": {clusterName: "invalid/root:invalid", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if err := handler.Write(testJSON, tc.clusterName); tc.expectErr {
				assert.Error(t, err)
				return
			}

			writtenData, err := os.ReadFile(filepath.Join(tempDir, tc.clusterName))
			assert.NoError(t, err)
			assert.Equal(t, string(writtenData), string(testJSON))
		})
	}
}
