package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndValidateYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "valid single document",
			yaml: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value
`,
		},
		{
			name: "valid document starting with ---",
			yaml: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
`,
		},
		{
			name:    "invalid YAML syntax",
			yaml:    `{not: valid: yaml: [`,
			wantErr: "invalid YAML",
		},
		{
			name: "missing apiVersion",
			yaml: `kind: ConfigMap
metadata:
  name: my-config
`,
			wantErr: "apiVersion is required",
		},
		{
			name: "missing kind",
			yaml: `apiVersion: v1
metadata:
  name: my-config
`,
			wantErr: "kind is required",
		},
		{
			name: "missing metadata",
			yaml: `apiVersion: v1
kind: ConfigMap
`,
			wantErr: "metadata is required",
		},
		{
			name: "missing metadata.name",
			yaml: `apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
`,
			wantErr: "metadata.name is required",
		},
		{
			name: "multi-document YAML",
			yaml: `apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second
`,
			wantErr: "multi-document YAML is not supported",
		},
		{
			name: "multi-document with leading separator",
			yaml: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second
`,
			wantErr: "multi-document YAML is not supported",
		},
		{
			name:    "empty YAML",
			yaml:    "",
			wantErr: "invalid YAML",
		},
		{
			name: "grouped apiVersion",
			yaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
spec:
  replicas: 1
`,
		},
		{
			name: "cluster-scoped resource without namespace",
			yaml: `apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
`,
		},
		{
			name: "non-string apiVersion",
			yaml: `apiVersion: 123
kind: ConfigMap
metadata:
  name: my-config
`,
			wantErr: "apiVersion is required and must be a string",
		},
		{
			name: "non-string kind",
			yaml: `apiVersion: v1
kind: 456
metadata:
  name: my-config
`,
			wantErr: "kind is required and must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAndValidateYAML(tt.yaml)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Verify parsed content preserves key fields
				assert.NotEmpty(t, result["apiVersion"])
				assert.NotEmpty(t, result["kind"])
				metadata, ok := result["metadata"].(map[string]any)
				require.True(t, ok)
				assert.NotEmpty(t, metadata["name"])
			}
		})
	}
}

func TestParseAndValidateYAML_ParsedContent(t *testing.T) {
	yamlStr := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
  labels:
    app: test
data:
  key1: value1
  key2: value2
`
	result, err := parseAndValidateYAML(yamlStr)
	require.NoError(t, err)

	assert.Equal(t, "v1", result["apiVersion"])
	assert.Equal(t, "ConfigMap", result["kind"])

	metadata := result["metadata"].(map[string]any)
	assert.Equal(t, "my-config", metadata["name"])
	assert.Equal(t, "default", metadata["namespace"])

	labels := metadata["labels"].(map[string]any)
	assert.Equal(t, "test", labels["app"])

	data := result["data"].(map[string]any)
	assert.Equal(t, "value1", data["key1"])
	assert.Equal(t, "value2", data["key2"])
}
