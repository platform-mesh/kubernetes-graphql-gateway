package clusteraccess_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
)

func TestInjectClusterMetadata(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name          string
		schemaJSON    []byte
		clusterAccess gatewayv1alpha1.ClusterAccess
		mockSetup     func(*mocks.MockClient)
		wantMetadata  map[string]interface{}
		wantErr       bool
		errContains   string
	}{
		{
			name:       "basic_metadata_injection",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "test-cluster",
			},
			wantErr: false,
		},
		{
			name:       "metadata_injection_with_custom_path",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					Path: "custom-path",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "custom-path",
			},
			wantErr: false,
		},
		{
			name:       "invalid_json",
			schemaJSON: []byte(`invalid json`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantErr:   true,
		},
		{
			name:       "empty_cluster_name_uses_empty_path",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: ""},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://example.com",
				"path": "",
			},
			wantErr: false,
		},
		{
			name:       "empty_path_empty_name_defaults_to_empty",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: ""},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://example.com",
					Path: "",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://example.com",
				"path": "",
			},
			wantErr: false,
		},
		{
			name:       "empty_host",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "no-host-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "",
				"path": "no-host-cluster",
			},
			wantErr: false,
		},
		{
			name:       "special_characters_in_name_and_path",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "special-chars_cluster.test"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://special.example.com",
					Path: "special/chars_path.test",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://special.example.com",
				"path": "special/chars_path.test",
			},
			wantErr: false,
		},
		{
			name:       "minimal_valid_json",
			schemaJSON: []byte(`{}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "minimal"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://minimal.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://minimal.example.com",
				"path": "minimal",
			},
			wantErr: false,
		},
		{
			name:       "long_complex_path",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "path-test"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://example.com",
					Path: "very/long/path/with/multiple/segments",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://example.com",
				"path": "very/long/path/with/multiple/segments",
			},
			wantErr: false,
		},
		{
			name:       "unicode_characters_in_name",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "üñíçødé-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://unicode.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantMetadata: map[string]interface{}{
				"host": "https://unicode.example.com",
				"path": "üñíçødé-cluster",
			},
			wantErr: false,
		},
		{
			name:       "malformed_json_brackets",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantErr:   true,
		},
		{
			name:       "empty_json",
			schemaJSON: []byte(``),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			result, err := clusteraccess.InjectClusterMetadata(t.Context(), tt.schemaJSON, tt.clusterAccess, mockClient, mockLogger)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Parse the result to verify metadata injection
			var resultData map[string]interface{}
			err = json.Unmarshal(result, &resultData)
			require.NoError(t, err)

			// Check that metadata was injected
			metadata, exists := resultData["x-cluster-metadata"]
			require.True(t, exists, "x-cluster-metadata should be present")

			metadataMap, ok := metadata.(map[string]interface{})
			require.True(t, ok, "x-cluster-metadata should be a map")

			// Verify expected metadata
			for key, expectedValue := range tt.wantMetadata {
				actualValue, exists := metadataMap[key]
				require.True(t, exists, "metadata key %s should be present", key)
				assert.Equal(t, expectedValue, actualValue, "metadata key %s should match", key)
			}
		})
	}
}

func TestInjectClusterMetadata_PathLogic(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())
	mockClient := mocks.NewMockClient(t)
	schemaJSON := []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`)

	t.Run("path_precedence_custom_over_name", func(t *testing.T) {
		clusterAccess := gatewayv1alpha1.ClusterAccess{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-name"},
			Spec: gatewayv1alpha1.ClusterAccessSpec{
				Host: "https://test.example.com",
				Path: "custom-path",
			},
		}

		result, err := clusteraccess.InjectClusterMetadata(t.Context(), schemaJSON, clusterAccess, mockClient, mockLogger)
		require.NoError(t, err)

		var resultData map[string]interface{}
		err = json.Unmarshal(result, &resultData)
		require.NoError(t, err)

		metadata := resultData["x-cluster-metadata"].(map[string]interface{})
		assert.Equal(t, "custom-path", metadata["path"])
	})

	t.Run("fallback_to_name_when_path_empty", func(t *testing.T) {
		clusterAccess := gatewayv1alpha1.ClusterAccess{
			ObjectMeta: metav1.ObjectMeta{Name: "fallback-name"},
			Spec: gatewayv1alpha1.ClusterAccessSpec{
				Host: "https://test.example.com",
				Path: "",
			},
		}

		result, err := clusteraccess.InjectClusterMetadata(t.Context(), schemaJSON, clusterAccess, mockClient, mockLogger)
		require.NoError(t, err)

		var resultData map[string]interface{}
		err = json.Unmarshal(result, &resultData)
		require.NoError(t, err)

		metadata := resultData["x-cluster-metadata"].(map[string]interface{})
		assert.Equal(t, "fallback-name", metadata["path"])
	})
}
