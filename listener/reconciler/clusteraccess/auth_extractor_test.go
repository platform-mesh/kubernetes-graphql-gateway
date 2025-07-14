package clusteraccess_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
)

func TestConfigureAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		auth        *gatewayv1alpha1.AuthConfig
		mockSetup   func(*mocks.MockClient)
		wantConfig  func(*rest.Config) *rest.Config
		wantErr     bool
		errContains string
	}{
		{
			name:      "nil_auth_config_does_nothing",
			auth:      nil,
			mockSetup: func(m *mocks.MockClient) {},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr: false,
		},
		{
			name: "bearer_token_auth_from_secret",
			auth: &gatewayv1alpha1.AuthConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name:      "auth-secret",
					Namespace: "test-ns",
					Key:       "token",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"token": []byte("test-bearer-token"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.BearerToken = "test-bearer-token"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "bearer_token_auth_defaults_to_default_namespace",
			auth: &gatewayv1alpha1.AuthConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name: "auth-secret",
					Key:  "token",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"token": []byte("test-bearer-token"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.BearerToken = "test-bearer-token"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "kubeconfig_auth_with_token",
			auth: &gatewayv1alpha1.AuthConfig{
				KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
					Name:      "kubeconfig-secret",
					Namespace: "test-ns",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				kubeconfigData := `
apiVersion: v1
kind: Config
current-context: test-context
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
users:
- name: test-user
  user:
    token: kubeconfig-token
clusters:
- name: test-cluster
  cluster:
    server: https://test.example.com
`
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"kubeconfig": []byte(kubeconfigData),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "kubeconfig-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.BearerToken = "kubeconfig-token"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "client_certificate_auth",
			auth: &gatewayv1alpha1.AuthConfig{
				ClientCertificateRef: &gatewayv1alpha1.ClientCertificateRef{
					Name:      "cert-secret",
					Namespace: "test-ns",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ncert-data\n-----END CERTIFICATE-----"),
						"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nkey-data\n-----END PRIVATE KEY-----"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "cert-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.TLSClientConfig.CertData = []byte("-----BEGIN CERTIFICATE-----\ncert-data\n-----END CERTIFICATE-----")
				expected.TLSClientConfig.KeyData = []byte("-----BEGIN PRIVATE KEY-----\nkey-data\n-----END PRIVATE KEY-----")
				return &expected
			},
			wantErr: false,
		},
		{
			name: "secret_not_found",
			auth: &gatewayv1alpha1.AuthConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name:      "missing-secret",
					Namespace: "test-ns",
					Key:       "token",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "missing-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					Return(errors.New("secret not found")).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr:     true,
			errContains: "failed to get auth secret",
		},
		{
			name: "auth_key_not_found_in_secret",
			auth: &gatewayv1alpha1.AuthConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name:      "auth-secret",
					Namespace: "test-ns",
					Key:       "missing-key",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"token": []byte("test-bearer-token"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr:     true,
			errContains: "auth key not found in secret",
		},
		{
			name: "invalid_kubeconfig",
			auth: &gatewayv1alpha1.AuthConfig{
				KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
					Name:      "kubeconfig-secret",
					Namespace: "test-ns",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"kubeconfig": []byte("invalid-yaml"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "kubeconfig-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr:     true,
			errContains: "failed to parse kubeconfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			config := &rest.Config{
				Host: "https://test.example.com",
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			}

			err := clusteraccess.ConfigureAuthentication(config, tt.auth, mockClient)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				expected := tt.wantConfig(config)
				assert.Equal(t, expected, config)
			}
		})
	}
}

func TestExtractAuthFromKubeconfig(t *testing.T) {
	tests := []struct {
		name        string
		authInfo    *api.AuthInfo
		wantConfig  func(*rest.Config) *rest.Config
		wantErr     bool
		errContains string
	}{
		{
			name: "token_auth",
			authInfo: &api.AuthInfo{
				Token: "test-token",
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.BearerToken = "test-token"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "client_certificate_data",
			authInfo: &api.AuthInfo{
				ClientCertificateData: []byte("cert-data"),
				ClientKeyData:         []byte("key-data"),
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.TLSClientConfig.CertData = []byte("cert-data")
				expected.TLSClientConfig.KeyData = []byte("key-data")
				return &expected
			},
			wantErr: false,
		},
		{
			name: "client_certificate_files",
			authInfo: &api.AuthInfo{
				ClientCertificate: "/path/to/cert.pem",
				ClientKey:         "/path/to/key.pem",
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.TLSClientConfig.CertFile = "/path/to/cert.pem"
				expected.TLSClientConfig.KeyFile = "/path/to/key.pem"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "basic_auth",
			authInfo: &api.AuthInfo{
				Username: "test-user",
				Password: "test-password",
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				expected := *config
				expected.Username = "test-user"
				expected.Password = "test-password"
				return &expected
			},
			wantErr: false,
		},
		{
			name: "token_file_not_implemented",
			authInfo: &api.AuthInfo{
				TokenFile: "/path/to/token",
			},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr:     true,
			errContains: "token file authentication not yet implemented",
		},
		{
			name:     "no_auth_info",
			authInfo: &api.AuthInfo{},
			wantConfig: func(config *rest.Config) *rest.Config {
				return config
			},
			wantErr:     true,
			errContains: "no valid authentication method found in kubeconfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &rest.Config{
				Host: "https://test.example.com",
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			}

			err := clusteraccess.ExtractAuthFromKubeconfig(config, tt.authInfo)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				expected := tt.wantConfig(config)
				assert.Equal(t, expected, config)
			}
		})
	}
}
