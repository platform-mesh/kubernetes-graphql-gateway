package kcp_test

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	"github.com/stretchr/testify/assert"
)

func TestStripAPIExportPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid_apiexport_url",
			input:    "https://kcp.example.com/services/apiexport/root:orgs:default/core.platform-mesh.io",
			expected: "https://kcp.example.com",
		},
		{
			name:     "apiexport_url_with_port",
			input:    "https://kcp.example.com:6443/services/apiexport/root/core.platform-mesh.io",
			expected: "https://kcp.example.com:6443",
		},
		{
			name:     "apiexport_url_with_query_params",
			input:    "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io?timeout=30s",
			expected: "https://kcp.example.com?timeout=30s",
		},
		{
			name:     "non_apiexport_url",
			input:    "https://kcp.example.com/clusters/root",
			expected: "https://kcp.example.com/clusters/root",
		},
		{
			name:     "base_url_no_path",
			input:    "https://kcp.example.com",
			expected: "https://kcp.example.com",
		},
		{
			name:     "invalid_url",
			input:    "not-a-valid-url",
			expected: "not-a-valid-url",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "url_with_fragment",
			input:    "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io#section",
			expected: "https://kcp.example.com#section",
		},
		{
			name:     "http_url",
			input:    "http://localhost:8080/services/apiexport/test/export",
			expected: "http://localhost:8080",
		},
		{
			name:     "apiexport_with_complex_workspace_path",
			input:    "https://kcp.example.com/services/apiexport/root:orgs:company:team:project/my.export.name",
			expected: "https://kcp.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kcp.StripAPIExportPathExported(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAPIExportRef(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedWorkspace string
		expectedExport    string
		expectErr         bool
		errContains       string
	}{
		{
			name:              "valid_simple_apiexport",
			input:             "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io",
			expectedWorkspace: kcp.RootClusterName,
			expectedExport:    "core.platform-mesh.io",
			expectErr:         false,
		},
		{
			name:              "valid_complex_workspace_path",
			input:             "https://kcp.example.com/services/apiexport/root:orgs:default/core.platform-mesh.io",
			expectedWorkspace: "root:orgs:default",
			expectedExport:    "core.platform-mesh.io",
			expectErr:         false,
		},
		{
			name:              "valid_with_port",
			input:             "https://kcp.example.com:6443/services/apiexport/root/core.platform-mesh.io",
			expectedWorkspace: kcp.RootClusterName,
			expectedExport:    "core.platform-mesh.io",
			expectErr:         false,
		},
		{
			name:              "valid_with_trailing_slash",
			input:             "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io/",
			expectedWorkspace: kcp.RootClusterName,
			expectedExport:    "core.platform-mesh.io",
			expectErr:         false,
		},
		{
			name:              "valid_with_query_params",
			input:             "https://kcp.example.com/services/apiexport/root/core.platform-mesh.io?timeout=30s",
			expectedWorkspace: kcp.RootClusterName,
			expectedExport:    "core.platform-mesh.io",
			expectErr:         false,
		},
		{
			name:              "valid_http_url",
			input:             "http://localhost:8080/services/apiexport/test/my-export",
			expectedWorkspace: "test",
			expectedExport:    "my-export",
			expectErr:         false,
		},
		{
			name:              "valid_complex_export_name",
			input:             "https://kcp.example.com/services/apiexport/root:orgs:company/io.kubernetes.core.v1",
			expectedWorkspace: "root:orgs:company",
			expectedExport:    "io.kubernetes.core.v1",
			expectErr:         false,
		},
		{
			name:        "invalid_url",
			input:       "not-a-valid-url",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "not_apiexport_url",
			input:       "https://kcp.example.com/clusters/root",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "missing_export_name",
			input:       "https://kcp.example.com/services/apiexport/root",
			expectErr:   true,
			errContains: "invalid APIExport URL format",
		},
		{
			name:        "missing_workspace_path",
			input:       "https://kcp.example.com/services/apiexport/",
			expectErr:   true,
			errContains: "invalid APIExport URL format",
		},
		{
			name:        "wrong_services_path",
			input:       "https://kcp.example.com/service/apiexport/root/export",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "wrong_apiexport_path",
			input:       "https://kcp.example.com/services/api-export/root/export",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "empty_string",
			input:       "",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "only_services",
			input:       "https://kcp.example.com/services/",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
		{
			name:        "malformed_path_structure",
			input:       "https://kcp.example.com/services/apiexport",
			expectErr:   true,
			errContains: "not an APIExport URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace, export, err := kcp.ExtractAPIExportRefExported(tt.input)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Empty(t, workspace)
				assert.Empty(t, export)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedWorkspace, workspace)
				assert.Equal(t, tt.expectedExport, export)
			}
		})
	}
}
