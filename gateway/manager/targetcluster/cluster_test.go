package targetcluster_test

import (
	"encoding/base64"
	"testing"

	"github.com/openmfp/golang-commons/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/targetcluster"
)

func TestBuildConfigFromMetadata(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	// Valid base64 encoded test data
	validCA := base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIICyDCCAbCgAwIBAgIBADANBgkqhkiG9w0BAQsFADA=\n-----END CERTIFICATE-----"))
	validToken := base64.StdEncoding.EncodeToString([]byte("test-token-123"))
	validCertData := base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIICert\n-----END CERTIFICATE-----"))
	validKeyData := base64.StdEncoding.EncodeToString([]byte("-----BEGIN PRIVATE KEY-----\nMIIKey\n-----END PRIVATE KEY-----"))

	// Valid kubeconfig (minimal but parseable)
	validKubeconfig := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com
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
	validKubeconfigB64 := base64.StdEncoding.EncodeToString([]byte(validKubeconfig))

	tests := []struct {
		name           string
		metadata       *targetcluster.ClusterMetadata
		expectError    bool
		errorContains  string
		validateConfig func(t *testing.T, config *rest.Config)
	}{
		{
			name: "basic_host_only",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.True(t, config.TLSClientConfig.Insecure)
				assert.Empty(t, config.BearerToken)
				assert.Nil(t, config.TLSClientConfig.CAData)
			},
		},
		{
			name: "with_valid_ca_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				CA: &targetcluster.CAMetadata{
					Data: validCA,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.False(t, config.TLSClientConfig.Insecure)
				assert.NotNil(t, config.TLSClientConfig.CAData)
				assert.Contains(t, string(config.TLSClientConfig.CAData), "BEGIN CERTIFICATE")
			},
		},
		{
			name: "with_invalid_ca_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				CA: &targetcluster.CAMetadata{
					Data: "invalid-base64-!@#$%",
				},
			},
			expectError:   true,
			errorContains: "failed to decode CA data",
		},
		{
			name: "with_empty_ca_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				CA: &targetcluster.CAMetadata{
					Data: "",
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.True(t, config.TLSClientConfig.Insecure)
				assert.Nil(t, config.TLSClientConfig.CAData)
			},
		},
		{
			name: "with_valid_token_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:  "token",
					Token: validToken,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Equal(t, "test-token-123", config.BearerToken)
			},
		},
		{
			name: "with_invalid_token_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:  "token",
					Token: "invalid-base64-!@#$%",
				},
			},
			expectError:   true,
			errorContains: "failed to decode token",
		},
		{
			name: "with_empty_token_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:  "token",
					Token: "",
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Empty(t, config.BearerToken)
			},
		},
		{
			name: "with_valid_kubeconfig_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:       "kubeconfig",
					Kubeconfig: validKubeconfigB64,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Equal(t, "kubeconfig-token-456", config.BearerToken)
			},
		},
		{
			name: "with_invalid_kubeconfig_base64",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:       "kubeconfig",
					Kubeconfig: "invalid-base64-!@#$%",
				},
			},
			expectError:   true,
			errorContains: "failed to decode kubeconfig",
		},
		{
			name: "with_invalid_kubeconfig_content",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:       "kubeconfig",
					Kubeconfig: base64.StdEncoding.EncodeToString([]byte("invalid yaml content")),
				},
			},
			expectError:   true,
			errorContains: "failed to parse kubeconfig",
		},
		{
			name: "with_empty_kubeconfig_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:       "kubeconfig",
					Kubeconfig: "",
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Empty(t, config.BearerToken)
			},
		},
		{
			name: "with_valid_client_cert_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: validCertData,
					KeyData:  validKeyData,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.NotNil(t, config.TLSClientConfig.CertData)
				assert.NotNil(t, config.TLSClientConfig.KeyData)
				assert.Contains(t, string(config.TLSClientConfig.CertData), "BEGIN CERTIFICATE")
				assert.Contains(t, string(config.TLSClientConfig.KeyData), "BEGIN PRIVATE KEY")
			},
		},
		{
			name: "with_invalid_client_cert_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: "invalid-base64-!@#$%",
					KeyData:  validKeyData,
				},
			},
			expectError:   true,
			errorContains: "failed to decode cert data",
		},
		{
			name: "with_invalid_client_key_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: validCertData,
					KeyData:  "invalid-base64-!@#$%",
				},
			},
			expectError:   true,
			errorContains: "failed to decode key data",
		},
		{
			name: "with_missing_client_cert_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: "",
					KeyData:  validKeyData,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Nil(t, config.TLSClientConfig.CertData)
				assert.Nil(t, config.TLSClientConfig.KeyData)
			},
		},
		{
			name: "with_missing_client_key_data",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: validCertData,
					KeyData:  "",
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Nil(t, config.TLSClientConfig.CertData)
				assert.Nil(t, config.TLSClientConfig.KeyData)
			},
		},
		{
			name: "with_unknown_auth_type",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				Auth: &targetcluster.AuthMetadata{
					Type:  "unknown",
					Token: validToken,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.Empty(t, config.BearerToken)
			},
		},
		{
			name: "with_ca_and_token_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				CA: &targetcluster.CAMetadata{
					Data: validCA,
				},
				Auth: &targetcluster.AuthMetadata{
					Type:  "token",
					Token: validToken,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.False(t, config.TLSClientConfig.Insecure)
				assert.NotNil(t, config.TLSClientConfig.CAData)
				assert.Equal(t, "test-token-123", config.BearerToken)
			},
		},
		{
			name: "with_ca_and_client_cert_auth",
			metadata: &targetcluster.ClusterMetadata{
				Host: "https://k8s.example.com",
				CA: &targetcluster.CAMetadata{
					Data: validCA,
				},
				Auth: &targetcluster.AuthMetadata{
					Type:     "clientCert",
					CertData: validCertData,
					KeyData:  validKeyData,
				},
			},
			expectError: false,
			validateConfig: func(t *testing.T, config *rest.Config) {
				assert.Equal(t, "https://k8s.example.com", config.Host)
				assert.False(t, config.TLSClientConfig.Insecure)
				assert.NotNil(t, config.TLSClientConfig.CAData)
				assert.NotNil(t, config.TLSClientConfig.CertData)
				assert.NotNil(t, config.TLSClientConfig.KeyData)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := targetcluster.BuildConfigFromMetadata(tt.metadata, log)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				if tt.validateConfig != nil {
					tt.validateConfig(t, config)
				}
			}
		})
	}
}
