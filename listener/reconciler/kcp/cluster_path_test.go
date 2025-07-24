package kcp_test

import (
	"context"
	"errors"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp"
)

func TestConfigForKCPCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		config      *rest.Config
		wantErr     bool
		errContains string
		wantHost    string
	}{
		{
			name:        "successful_config_creation",
			clusterName: "test-cluster",
			config: &rest.Config{
				Host: "https://api.example.com:443",
			},
			wantErr:  false,
			wantHost: "https://api.example.com:443/clusters/test-cluster",
		},
		{
			name:        "nil_config_returns_error",
			clusterName: "test-cluster",
			config:      nil,
			wantErr:     true,
			errContains: "config cannot be nil",
		},
		{
			name:        "invalid_host_url_returns_error",
			clusterName: "test-cluster",
			config: &rest.Config{
				Host: "://invalid-url",
			},
			wantErr:     true,
			errContains: "failed to parse host URL",
		},
		{
			name:        "config_with_existing_path",
			clusterName: "workspace-1",
			config: &rest.Config{
				Host: "https://kcp.example.com/clusters/root",
			},
			wantErr:  false,
			wantHost: "https://kcp.example.com/clusters/workspace-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kcp.ConfigForKCPClusterExported(tt.clusterName, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantHost, got.Host)
				// Ensure original config is not modified
				assert.NotEqual(t, tt.config.Host, got.Host)
			}
		})
	}
}

func TestNewClusterPathResolver(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name        string
		config      *rest.Config
		scheme      *runtime.Scheme
		wantErr     bool
		errContains string
	}{
		{
			name: "successful_creation",
			config: &rest.Config{
				Host: "https://api.example.com",
			},
			scheme:  scheme,
			wantErr: false,
		},
		{
			name:        "nil_config_returns_error",
			config:      nil,
			scheme:      scheme,
			wantErr:     true,
			errContains: "config cannot be nil",
		},
		{
			name: "nil_scheme_returns_error",
			config: &rest.Config{
				Host: "https://api.example.com",
			},
			scheme:      nil,
			wantErr:     true,
			errContains: "scheme should not be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kcp.NewClusterPathResolverExported(tt.config, tt.scheme)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.config, got.Config)
				assert.Equal(t, tt.scheme, got.Scheme)
			}
		})
	}
}

func TestClusterPathResolverProvider_ClientForCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	baseConfig := &rest.Config{
		Host: "https://api.example.com",
	}

	tests := []struct {
		name          string
		clusterName   string
		clientFactory func(config *rest.Config, options client.Options) (client.Client, error)
		wantErr       bool
		errContains   string
	}{
		{
			name:        "successful_client_creation",
			clusterName: "test-cluster",
			clientFactory: func(config *rest.Config, options client.Options) (client.Client, error) {
				// Verify that the config was properly modified
				assert.Equal(t, "https://api.example.com/clusters/test-cluster", config.Host)
				return mocks.NewMockClient(t), nil
			},
			wantErr: false,
		},
		{
			name:        "client_factory_error",
			clusterName: "test-cluster",
			clientFactory: func(config *rest.Config, options client.Options) (client.Client, error) {
				return nil, errors.New("client creation failed")
			},
			wantErr:     true,
			errContains: "client creation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := kcp.NewClusterPathResolverProviderWithFactory(baseConfig, scheme, tt.clientFactory)

			got, err := resolver.ClientForCluster(tt.clusterName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestPathForCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		mockSetup   func(*mocks.MockClient)
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "root_cluster_returns_root",
			clusterName: "root",
			mockSetup:   func(m *mocks.MockClient) {},
			want:        "root",
			wantErr:     false,
		},
		{
			name:        "successful_path_extraction",
			clusterName: "workspace-1",
			mockSetup: func(m *mocks.MockClient) {
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:workspace-1",
						},
					},
				}
				m.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()
			},
			want:    "root:org:workspace-1",
			wantErr: false,
		},
		{
			name:        "cluster_is_deleted",
			clusterName: "deleted-workspace",
			mockSetup: func(m *mocks.MockClient) {
				now := metav1.Now()
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:deleted-workspace",
						},
						DeletionTimestamp: &now,
					},
				}
				m.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()
			},
			want:        "root:org:deleted-workspace",
			wantErr:     true,
			errContains: "cluster is deleted",
		},
		{
			name:        "missing_path_annotation",
			clusterName: "no-path-workspace",
			mockSetup: func(m *mocks.MockClient) {
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "cluster",
						Annotations: map[string]string{},
					},
				}
				m.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()
			},
			want:        "",
			wantErr:     true,
			errContains: "failed to get cluster path from kcp.io/path annotation",
		},
		{
			name:        "client_get_error",
			clusterName: "error-workspace",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Return(errors.New("API server error")).Once()
			},
			want:        "",
			wantErr:     true,
			errContains: "failed to get logicalcluster resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			got, err := kcp.PathForClusterExported(tt.clusterName, mockClient)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				if tt.name == "cluster_is_deleted" {
					// Special case: when cluster is deleted, we still return the path but also an error
					assert.Equal(t, tt.want, got)
					assert.ErrorIs(t, err, kcp.ErrClusterIsDeletedExported)
				} else {
					assert.Equal(t, tt.want, got)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("error_variables", func(t *testing.T) {
		assert.Equal(t, "config cannot be nil", kcp.ErrNilConfigExported.Error())
		assert.Equal(t, "scheme should not be nil", kcp.ErrNilSchemeExported.Error())
		assert.Equal(t, "failed to get cluster config", kcp.ErrGetClusterConfigExported.Error())
		assert.Equal(t, "failed to get logicalcluster resource", kcp.ErrGetLogicalClusterExported.Error())
		assert.Equal(t, "failed to get cluster path from kcp.io/path annotation", kcp.ErrMissingPathAnnotationExported.Error())
		assert.Equal(t, "failed to parse rest config's Host URL", kcp.ErrParseHostURLExported.Error())
		assert.Equal(t, "cluster is deleted", kcp.ErrClusterIsDeletedExported.Error())
	})
}
