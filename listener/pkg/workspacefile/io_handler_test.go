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

	handler, err := NewIOHandler(tempDir)
	assert.NoError(t, err)

	tests := map[string]struct {
		clusterName string
		expectErr   bool
	}{
		"valid_file":        {clusterName: validClusterName, expectErr: false},
		"non_existent_file": {clusterName: "root:non-existent", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := handler.Read(t.Context(), tc.clusterName)
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
	handler, err := NewIOHandler(tempDir)
	assert.NoError(t, err)

	tests := map[string]struct {
		clusterName string
		expectErr   bool
	}{
		"valid_write":         {clusterName: "root:sap:openmfp", expectErr: false},
		"subdirectory_path":   {clusterName: "virtual-workspace/api-export-ws", expectErr: false},
		"nested_subdirectory": {clusterName: "some/nested/path/workspace", expectErr: false},
		"invalid_file_chars":  {clusterName: "invalid\x00name", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if err := handler.Write(t.Context(), testJSON, tc.clusterName); tc.expectErr {
				assert.Error(t, err)
				return
			}

			writtenData, err := os.ReadFile(filepath.Join(tempDir, tc.clusterName))
			assert.NoError(t, err)
			assert.Equal(t, string(writtenData), string(testJSON))
		})
	}
}

func TestDelete(t *testing.T) {
	tempDir := t.TempDir()
	handler, err := NewIOHandler(tempDir)
	assert.NoError(t, err)

	existing := "root:sap:openmfp"
	nested := filepath.Join("some", "nested", "path", "workspace")

	err = os.WriteFile(filepath.Join(tempDir, existing), testJSON, 0o644)
	assert.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempDir, filepath.Dir(nested)), 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, nested), testJSON, 0o644)
	assert.NoError(t, err)

	tests := map[string]struct {
		clusterName string
		expectErr   bool
	}{
		"existing_file":     {clusterName: existing, expectErr: false},
		"nested_file":       {clusterName: nested, expectErr: false},
		"non_existent_file": {clusterName: "does/not/exist", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := handler.Delete(t.Context(), tc.clusterName)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			_, statErr := os.Stat(filepath.Join(tempDir, tc.clusterName))
			assert.Error(t, statErr)
		})
	}
}
