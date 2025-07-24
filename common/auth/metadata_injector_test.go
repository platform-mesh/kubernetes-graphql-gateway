package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/base64"

	"github.com/openmfp/golang-commons/logger/testlogger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInjectKCPMetadataFromEnv(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

	// Create a temporary kubeconfig for testing
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")

	kubeconfigContent := `
apiVersion: v1
kind: Config
current-context: test-context
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
clusters:
- name: test-cluster
  cluster:
    server: https://kcp.api.portal.cc-d1.showroom.apeirora.eu:443
    certificate-authority-data: LS0tLS1CRUdJTi0tLS0t
users:
- name: test-user
  user:
    token: test-token
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	originalKubeconfig := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", originalKubeconfig)
	os.Setenv("KUBECONFIG", kubeconfigPath)

	tests := []struct {
		name         string
		schemaJSON   []byte
		clusterPath  string
		expectedHost string
		expectError  bool
	}{
		{
			name: "successful_injection",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": {
						"type": "object",
						"properties": {
							"metadata": {
								"type": "object"
							}
						}
					}
				}
			}`),
			clusterPath:  "root:test",
			expectedHost: "https://kcp.api.portal.cc-d1.showroom.apeirora.eu:443",
			expectError:  false,
		},
		{
			name: "invalid_json",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": invalid-json
				}
			}`),
			clusterPath: "root:test",
			expectError: true,
		},
	}

	// Add test for host override (virtual workspace)
	t.Run("with_host_override", func(t *testing.T) {
		overrideURL := "https://kcp.api.portal.cc-d1.showroom.apeirora.eu:443/services/contentconfigurations"
		schemaJSON := []byte(`{
			"definitions": {
				"test.resource": {
					"type": "object",
					"properties": {
						"metadata": {
							"type": "object"
						}
					}
				}
			}
		}`)

		result, err := InjectKCPMetadataFromEnv(schemaJSON, "virtual-workspace/custom-ws", log, overrideURL)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Parse the result to verify metadata injection
		var resultData map[string]interface{}
		err = json.Unmarshal(result, &resultData)
		require.NoError(t, err)

		// Check that metadata was injected with override host
		metadata, exists := resultData["x-cluster-metadata"]
		require.True(t, exists, "x-cluster-metadata should be present")

		metadataMap, ok := metadata.(map[string]interface{})
		require.True(t, ok, "x-cluster-metadata should be a map")

		// Verify override host is used
		host, exists := metadataMap["host"]
		require.True(t, exists, "host should be present")
		assert.Equal(t, overrideURL, host, "host should be the override URL")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := InjectKCPMetadataFromEnv(tt.schemaJSON, tt.clusterPath, log)

			if tt.expectError {
				assert.Error(t, err)
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

			// Verify host
			host, exists := metadataMap["host"]
			require.True(t, exists, "host should be present")
			assert.Equal(t, tt.expectedHost, host)

			// Verify path
			path, exists := metadataMap["path"]
			require.True(t, exists, "path should be present")
			assert.Equal(t, tt.clusterPath, path)

			// Verify auth
			auth, exists := metadataMap["auth"]
			require.True(t, exists, "auth should be present")

			authMap, ok := auth.(map[string]interface{})
			require.True(t, ok, "auth should be a map")

			authType, exists := authMap["type"]
			require.True(t, exists, "auth type should be present")
			assert.Equal(t, "kubeconfig", authType)

			kubeconfig, exists := authMap["kubeconfig"]
			require.True(t, exists, "kubeconfig should be present")
			assert.NotEmpty(t, kubeconfig, "kubeconfig should not be empty")

			// Verify CA data (if present)
			if ca, exists := metadataMap["ca"]; exists {
				caMap, ok := ca.(map[string]interface{})
				require.True(t, ok, "ca should be a map")

				caData, exists := caMap["data"]
				require.True(t, exists, "ca data should be present")
				assert.NotEmpty(t, caData, "ca data should not be empty")
			}
		})
	}
}

func TestInjectClusterMetadata(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

	tests := []struct {
		name         string
		schemaJSON   []byte
		config       MetadataInjectionConfig
		expectedHost string
		expectedPath string
		expectError  bool
	}{
		{
			name: "basic_metadata_injection",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": {
						"type": "object",
						"properties": {
							"metadata": {
								"type": "object"
							}
						}
					}
				}
			}`),
			config: MetadataInjectionConfig{
				Host: "https://test-cluster.example.com:6443",
				Path: "test-cluster",
			},
			expectedHost: "https://test-cluster.example.com:6443",
			expectedPath: "test-cluster",
			expectError:  false,
		},
		{
			name: "with_host_override",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": {
						"type": "object"
					}
				}
			}`),
			config: MetadataInjectionConfig{
				Host:         "https://original.example.com:6443",
				Path:         "virtual-workspace/test",
				HostOverride: "https://override.example.com:6443/services/test",
			},
			expectedHost: "https://override.example.com:6443/services/test",
			expectedPath: "virtual-workspace/test",
			expectError:  false,
		},
		{
			name: "virtual_workspace_path_stripping",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": {
						"type": "object"
					}
				}
			}`),
			config: MetadataInjectionConfig{
				Host: "https://kcp.example.com:6443/services/apiexport/some/path",
				Path: "test-workspace",
			},
			expectedHost: "https://kcp.example.com:6443", // Should be stripped
			expectedPath: "test-workspace",
			expectError:  false,
		},
		{
			name: "invalid_json",
			schemaJSON: []byte(`{
				"definitions": {
					"test.resource": invalid-json
				}
			}`),
			config: MetadataInjectionConfig{
				Host: "https://test.example.com:6443",
				Path: "test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use nil client since we're not testing auth/CA extraction here
			result, err := InjectClusterMetadata(t.Context(), tt.schemaJSON, tt.config, nil, log)

			if tt.expectError {
				assert.Error(t, err)
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

			// Verify host
			host, exists := metadataMap["host"]
			require.True(t, exists, "host should be present")
			assert.Equal(t, tt.expectedHost, host)

			// Verify path
			path, exists := metadataMap["path"]
			require.True(t, exists, "path should be present")
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

func TestExtractKubeconfigFromEnv(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger

	tests := []struct {
		name          string
		setupEnv      func() (cleanup func())
		expectedHost  string
		expectError   bool
		errorContains string
	}{
		{
			name: "from_env_variable",
			setupEnv: func() func() {
				tempDir := t.TempDir()
				kubeconfigPath := filepath.Join(tempDir, "config")

				kubeconfigContent := `
apiVersion: v1
kind: Config
current-context: test-context
contexts:
- name: test-context
  context:
    cluster: test-cluster
clusters:
- name: test-cluster
  cluster:
    server: https://test.example.com:6443
`

				err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644)
				require.NoError(t, err)

				original := os.Getenv("KUBECONFIG")
				os.Setenv("KUBECONFIG", kubeconfigPath)

				return func() {
					os.Setenv("KUBECONFIG", original)
				}
			},
			expectedHost: "https://test.example.com:6443",
			expectError:  false,
		},
		{
			name: "file_not_found",
			setupEnv: func() func() {
				original := os.Getenv("KUBECONFIG")
				os.Setenv("KUBECONFIG", "/non/existent/path")

				return func() {
					os.Setenv("KUBECONFIG", original)
				}
			},
			expectError:   true,
			errorContains: "kubeconfig file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			kubeconfigData, host, err := extractKubeconfigFromEnv(log)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, kubeconfigData)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

func TestStripVirtualWorkspacePath(t *testing.T) {
	tests := []struct {
		name     string
		hostURL  string
		expected string
	}{
		{
			name:     "virtual_workspace_path",
			hostURL:  "https://kcp.example.com:6443/services/apiexport/some/path",
			expected: "https://kcp.example.com:6443",
		},
		{
			name:     "no_virtual_workspace_path",
			hostURL:  "https://kcp.example.com:6443",
			expected: "https://kcp.example.com:6443",
		},
		{
			name:     "different_path",
			hostURL:  "https://kcp.example.com:6443/api/v1/clusters",
			expected: "https://kcp.example.com:6443/api/v1/clusters",
		},
		{
			name:     "invalid_url",
			hostURL:  "not-a-valid-url",
			expected: "not-a-valid-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripVirtualWorkspacePath(tt.hostURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAuthDataForMetadata(t *testing.T) {
	ctx := t.Context()

	t.Run("nil_auth_config", func(t *testing.T) {
		result, err := extractAuthDataForMetadata(ctx, nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty_auth_config", func(t *testing.T) {
		auth := &gatewayv1alpha1.AuthConfig{}
		result, err := extractAuthDataForMetadata(ctx, auth, nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("secret_ref_token_auth", func(t *testing.T) {
		// Create mock secret with token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"token": []byte("test-token-123"),
			},
		}

		// Create fake client with the secret
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		// Create auth config
		auth := &gatewayv1alpha1.AuthConfig{
			SecretRef: &gatewayv1alpha1.SecretRef{
				Name:      "test-token-secret",
				Namespace: "test-namespace",
				Key:       "token",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "token", result["type"])
		assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("test-token-123")), result["token"])
	})

	t.Run("kubeconfig_secret_ref", func(t *testing.T) {
		kubeconfigData := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test.example.com
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: kubeconfig-token-456
`

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeconfig-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"kubeconfig": []byte(kubeconfigData),
			},
		}

		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		auth := &gatewayv1alpha1.AuthConfig{
			KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
				Name:      "kubeconfig-secret",
				Namespace: "test-namespace",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "kubeconfig", result["type"])
		assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(kubeconfigData)), result["kubeconfig"])
	})

	t.Run("client_certificate_ref", func(t *testing.T) {
		certData := []byte("-----BEGIN CERTIFICATE-----\nMIICert\n-----END CERTIFICATE-----")
		keyData := []byte("-----BEGIN PRIVATE KEY-----\nMIIKey\n-----END PRIVATE KEY-----")

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"tls.crt": certData,
				"tls.key": keyData,
			},
		}

		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		auth := &gatewayv1alpha1.AuthConfig{
			ClientCertificateRef: &gatewayv1alpha1.ClientCertificateRef{
				Name:      "cert-secret",
				Namespace: "test-namespace",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "clientCert", result["type"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(certData), result["certData"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(keyData), result["keyData"])
	})

	t.Run("secret_not_found", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		auth := &gatewayv1alpha1.AuthConfig{
			SecretRef: &gatewayv1alpha1.SecretRef{
				Name:      "non-existent-secret",
				Namespace: "test-namespace",
				Key:       "token",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get auth secret")
		assert.Nil(t, result)
	})

	t.Run("kubeconfig_secret_not_found", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		auth := &gatewayv1alpha1.AuthConfig{
			KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
				Name:      "missing-kubeconfig",
				Namespace: "test-namespace",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get kubeconfig secret")
		assert.Nil(t, result)
	})

	t.Run("client_certificate_secret_not_found", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, gatewayv1alpha1.AddToScheme(scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		auth := &gatewayv1alpha1.AuthConfig{
			ClientCertificateRef: &gatewayv1alpha1.ClientCertificateRef{
				Name:      "missing-cert-secret",
				Namespace: "test-namespace",
			},
		}

		result, err := extractAuthDataForMetadata(ctx, auth, fakeClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get client certificate secret")
		assert.Nil(t, result)
	})
}
