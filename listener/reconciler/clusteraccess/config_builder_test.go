package clusteraccess_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
)

func TestBuildTargetClusterConfigFromTyped(t *testing.T) {
	tests := []struct {
		name          string
		clusterAccess gatewayv1alpha1.ClusterAccess
		mockSetup     func(*mocks.MockClient)
		wantConfig    *rest.Config
		wantCluster   string
		wantErr       bool
		errContains   string
	}{
		{
			name: "basic_config_without_CA_or_auth",
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
				},
			},
			mockSetup: func(m *mocks.MockClient) {},
			wantConfig: &rest.Config{
				Host: "https://test-cluster.example.com",
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
			wantCluster: "test-cluster",
			wantErr:     false,
		},
		{
			name: "config_with_missing_host",
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec:       gatewayv1alpha1.ClusterAccessSpec{},
			},
			mockSetup:   func(m *mocks.MockClient) {},
			wantConfig:  nil,
			wantCluster: "",
			wantErr:     true,
			errContains: "host field not found in ClusterAccess spec",
		},
		{
			name: "config_with_CA_secret",
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					CA: &gatewayv1alpha1.CAConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "ca-secret",
							Namespace: "default",
							Key:       "ca.crt",
						},
					},
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"ca.crt": []byte("-----BEGIN CERTIFICATE-----\ntest-ca-data\n-----END CERTIFICATE-----"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "ca-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: &rest.Config{
				Host: "https://test-cluster.example.com",
				TLSClientConfig: rest.TLSClientConfig{
					CAData:   []byte("-----BEGIN CERTIFICATE-----\ntest-ca-data\n-----END CERTIFICATE-----"),
					Insecure: false,
				},
			},
			wantCluster: "test-cluster",
			wantErr:     false,
		},
		{
			name: "config_with_token_auth",
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					Auth: &gatewayv1alpha1.AuthConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "auth-secret",
							Namespace: "default",
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
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "auth-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			wantConfig: &rest.Config{
				Host:        "https://test-cluster.example.com",
				BearerToken: "test-token",
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
			wantCluster: "test-cluster",
			wantErr:     false,
		},
		{
			name: "ca_secret_not_found",
			clusterAccess: gatewayv1alpha1.ClusterAccess{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: gatewayv1alpha1.ClusterAccessSpec{
					Host: "https://test-cluster.example.com",
					CA: &gatewayv1alpha1.CAConfig{
						SecretRef: &gatewayv1alpha1.SecretRef{
							Name:      "missing-secret",
							Namespace: "default",
							Key:       "ca.crt",
						},
					},
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "missing-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
					Return(errors.New("secret not found")).Once()
			},
			wantConfig:  nil,
			wantCluster: "",
			wantErr:     true,
			errContains: "failed to extract CA data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			gotConfig, gotCluster, err := clusteraccess.BuildTargetClusterConfigFromTyped(t.Context(), tt.clusterAccess, mockClient)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, gotConfig)
				assert.Empty(t, gotCluster)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfig, gotConfig)
				assert.Equal(t, tt.wantCluster, gotCluster)
			}
		})
	}
}

func TestExtractCAData(t *testing.T) {
	tests := []struct {
		name      string
		ca        *gatewayv1alpha1.CAConfig
		mockSetup func(*mocks.MockClient)
		want      []byte
		wantErr   bool
	}{
		{
			name:      "nil_ca_config_returns_nil",
			ca:        nil,
			mockSetup: func(m *mocks.MockClient) {},
			want:      nil,
			wantErr:   false,
		},
		{
			name: "extract_from_secret",
			ca: &gatewayv1alpha1.CAConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name:      "ca-secret",
					Namespace: "test-ns",
					Key:       "ca.crt",
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
			want:    []byte("test-ca-data"),
			wantErr: false,
		},
		{
			name: "extract_from_configmap",
			ca: &gatewayv1alpha1.CAConfig{
				ConfigMapRef: &gatewayv1alpha1.ConfigMapRef{
					Name:      "ca-configmap",
					Namespace: "test-ns",
					Key:       "ca.crt",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				configMap := &corev1.ConfigMap{
					Data: map[string]string{
						"ca.crt": "test-ca-data",
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "ca-configmap", Namespace: "test-ns"}, mock.AnythingOfType("*v1.ConfigMap")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						configMapObj := obj.(*corev1.ConfigMap)
						*configMapObj = *configMap
						return nil
					}).Once()
			},
			want:    []byte("test-ca-data"),
			wantErr: false,
		},
		{
			name: "secret_key_not_found",
			ca: &gatewayv1alpha1.CAConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name:      "ca-secret",
					Namespace: "test-ns",
					Key:       "missing-key",
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "secret_defaults_to_default_namespace",
			ca: &gatewayv1alpha1.CAConfig{
				SecretRef: &gatewayv1alpha1.SecretRef{
					Name: "ca-secret",
					Key:  "ca.crt",
				},
			},
			mockSetup: func(m *mocks.MockClient) {
				secret := &corev1.Secret{
					Data: map[string][]byte{
						"ca.crt": []byte("test-ca-data"),
					},
				}
				m.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "ca-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						secretObj := obj.(*corev1.Secret)
						*secretObj = *secret
						return nil
					}).Once()
			},
			want:    []byte("test-ca-data"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			got, err := clusteraccess.ExtractCAData(t.Context(), tt.ca, mockClient)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
