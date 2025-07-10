package clusteraccess_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
			name:       "metadata_injection_with_CA_secret",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					CA: &gatewayv1alpha1.CAConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "ca-secret",
							Namespace: "test-ns",
							Key:       "ca.crt",
						},
					},
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"ca.crt": []byte("test-ca-data"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "ca-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "test-cluster",
				"ca": map[string]interface{}{
					"data": base64.StdEncoding.EncodeToString([]byte("test-ca-data")),
				},
			},
			wantErr: false,
		},
		{
			name:       "metadata_injection_with_auth_secret",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					Auth: &gatewayv1alpha1.AuthConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "auth-secret",
							Namespace: "test-ns",
							Key:       "token",
						},
					},
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"token": []byte("test-token"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "test-cluster",
				"auth": map[string]interface{}{
					"type":  "token",
					"token": base64.StdEncoding.EncodeToString([]byte("test-token")),
				},
			},
			wantErr: false,
		},
		{
			name:       "metadata_injection_with_kubeconfig",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					Auth: &gatewayv1alpha1.AuthConfig{
						KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
							Name:      "kubeconfig-secret",
							Namespace: "test-ns",
						},
					},
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
    token: test-token
clusters:
- name: test-cluster
  cluster:
    server: https://test.example.com
    certificate-authority-data: ` + base64.StdEncoding.EncodeToString([]byte("ca-from-kubeconfig"))
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
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "test-cluster",
				"auth": map[string]interface{}{
					"type": "kubeconfig",
					"kubeconfig": base64.StdEncoding.EncodeToString([]byte(`
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
    token: test-token
clusters:
- name: test-cluster
  cluster:
    server: https://test.example.com
    certificate-authority-data: ` + base64.StdEncoding.EncodeToString([]byte("ca-from-kubeconfig")))),
				},
				"ca": map[string]interface{}{
					"data": base64.StdEncoding.EncodeToString([]byte("ca-from-kubeconfig")),
				},
			},
			wantErr: false,
		},
		{
			name:       "invalid_schema_JSON",
			schemaJSON: []byte(`invalid-json`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup:   func(m *mocks.MockClient) {},
			wantErr:     true,
			errContains: "failed to parse schema JSON",
		},
		{
			name:       "auth_secret_not_found_(warning_logged,_continues)",
			schemaJSON: []byte(`{"openapi": "3.0.0", "info": {"title": "Test"}}`),
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					Auth: &gatewayv1alpha1.AuthConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "missing-secret",
							Namespace: "test-ns",
							Key:       "token",
						},
					},
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "missing-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					Return(errors.New("secret not found")).Once()
			},
			wantMetadata: map[string]interface{}{
				"host": "https://test-cluster.example.com",
				"path": "test-cluster",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			got, err := clusteraccess.InjectClusterMetadata(tt.schemaJSON, tt.clusterAccess, mockClient, mockLogger)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)

				var result map[string]interface{}
				err := json.Unmarshal(got, &result)
				assert.NoError(t, err)

				metadata, exists := result["x-cluster-metadata"]
				assert.True(t, exists, "x-cluster-metadata should exist")

				metadataMap, ok := metadata.(map[string]interface{})
				assert.True(t, ok, "x-cluster-metadata should be a map")

				for key, expected := range tt.wantMetadata {
					actual, exists := metadataMap[key]
					assert.True(t, exists, "Expected metadata key %s should exist", key)
					assert.Equal(t, expected, actual, "Metadata key %s should match", key)
				}
			}
		})
	}
}

func TestExtractAuthDataForMetadata(t *testing.T) {
	tests := []struct {
		name      string
		auth      *gatewayv1alpha1.AuthConfig
		mockSetup func(*mocks.MockClient)
		want      map[string]interface{}
		wantErr   bool
	}{
		{
			name:      "nil_auth_returns_nil",
			auth:      nil,
			mockSetup: func(m *mocks.MockClient) {},
			want:      nil,
			wantErr:   false,
		},
		{
			name: "token_auth_from_secret",
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
						"token": []byte("test-token"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			want: map[string]interface{}{
				"type":  "token",
				"token": base64.StdEncoding.EncodeToString([]byte("test-token")),
			},
			wantErr: false,
		},
		{
			name: "kubeconfig_auth",
			auth: &gatewayv1alpha1.AuthConfig{
				KubeconfigSecretRef: &gatewayv1alpha1.KubeconfigSecretRef{
					Name:      "kubeconfig-secret",
					Namespace: "test-ns",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				kubeconfigData := `apiVersion: v1
kind: Config`
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
			want: map[string]interface{}{
				"type": "kubeconfig",
				"kubeconfig": base64.StdEncoding.EncodeToString([]byte(`apiVersion: v1
kind: Config`)),
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
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "cert-secret", Namespace: "test-ns"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			want: map[string]interface{}{
				"type":     "clientCert",
				"certData": base64.StdEncoding.EncodeToString([]byte("cert-data")),
				"keyData":  base64.StdEncoding.EncodeToString([]byte("key-data")),
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
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			got, err := clusteraccess.ExtractAuthDataForMetadata(tt.auth, mockClient)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExtractCAFromKubeconfig(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name          string
		kubeconfigB64 string
		want          []byte
	}{
		{
			name: "CA_data_from_kubeconfig",
			kubeconfigB64: base64.StdEncoding.EncodeToString([]byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ` + base64.StdEncoding.EncodeToString([]byte("test-ca-data")) + `
    server: https://test.example.com
  name: test-cluster
current-context: test-context
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
users:
- name: test-user
  user:
    token: test-token
`)),
			want: []byte("test-ca-data"),
		},
		{
			name: "no_CA_data_in_kubeconfig",
			kubeconfigB64: base64.StdEncoding.EncodeToString([]byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test.example.com
  name: test-cluster
current-context: test-context
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
users:
- name: test-user
  user:
    token: test-token
`)),
			want: nil,
		},
		{
			name:          "invalid_kubeconfig",
			kubeconfigB64: "invalid-base64",
			want:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clusteraccess.ExtractCAFromKubeconfig(tt.kubeconfigB64, mockLogger)
			assert.Equal(t, tt.want, got)
		})
	}
}
