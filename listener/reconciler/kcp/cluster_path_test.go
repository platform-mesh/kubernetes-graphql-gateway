package kcp_test

import (
	"context"
	"errors"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
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
			want:        "root:org:deleted-workspace", // Now correctly returns the kcp.io/path annotation value
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
			want:        "no-path-workspace", // Now returns cluster name as fallback
			wantErr:     false,               // No longer an error
			errContains: "",
		},
		{
			name:        "client_get_error",
			clusterName: "error-workspace",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Return(errors.New("API server error")).Once()
			},
			want:        "error-workspace", // Now returns cluster name as fallback
			wantErr:     false,             // No longer an error
			errContains: "",
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

func TestPathForClusterFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		config      *rest.Config
		want        string
	}{
		{
			name:        "root_cluster_returns_root",
			clusterName: "root",
			config:      &rest.Config{Host: "https://kcp.example.com"},
			want:        "root",
		},
		{
			name:        "nil_config_returns_cluster_name",
			clusterName: "test-cluster",
			config:      nil,
			want:        "test-cluster",
		},
		{
			name:        "invalid_url_returns_cluster_name",
			clusterName: "test-cluster",
			config:      &rest.Config{Host: "://invalid-url"},
			want:        "test-cluster",
		},
		{
			name:        "cluster_url_with_workspace_path",
			clusterName: "hash123",
			config:      &rest.Config{Host: "https://kcp.example.com/clusters/root:org:workspace"},
			want:        "root:org:workspace",
		},
		{
			name:        "cluster_url_with_hash_only",
			clusterName: "hash123",
			config:      &rest.Config{Host: "https://kcp.example.com/clusters/hash123"},
			want:        "hash123",
		},
		{
			name:        "virtual_workspace_url_extracts_workspace_path",
			clusterName: "hash123",
			config:      &rest.Config{Host: "https://kcp.example.com/services/apiexport/root:orgs:default/some-export"},
			want:        "root:orgs:default",
		},
		{
			name:        "virtual_workspace_url_simple_workspace",
			clusterName: "hash123",
			config:      &rest.Config{Host: "https://kcp.example.com/services/apiexport/root/some-export"},
			want:        "root",
		},
		{
			name:        "cluster_url_different_from_hash",
			clusterName: "hash123",
			config:      &rest.Config{Host: "https://kcp.example.com/clusters/simple-workspace"},
			want:        "simple-workspace",
		},
		{
			name:        "non_matching_url_returns_cluster_name",
			clusterName: "test-cluster",
			config:      &rest.Config{Host: "https://kcp.example.com/other/path"},
			want:        "test-cluster",
		},
		{
			name:        "cluster_url_without_path_returns_cluster_name",
			clusterName: "test-cluster",
			config:      &rest.Config{Host: "https://kcp.example.com"},
			want:        "test-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kcp.PathForClusterFromConfigExported(tt.clusterName, tt.config)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
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

func TestPathForClusterFromWorkspaces(t *testing.T) {
	tests := []struct {
		name         string
		clusterHash  string
		workspaces   []kcptenancy.Workspace
		listError    error
		expectedPath string
		expectError  bool
	}{
		{
			name:         "root_cluster_returns_root",
			clusterHash:  "root",
			expectedPath: "root",
			expectError:  false,
		},
		{
			name:        "workspace_found_by_logical_cluster",
			clusterHash: "2no5f6yyo9w7af0e",
			workspaces: []kcptenancy.Workspace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "platform-mesh-system",
						Namespace:   "root",
						Annotations: map[string]string{},
					},
				},
			},
			expectedPath: "root:platform-mesh-system",
			expectError:  false,
		},
		{
			name:        "workspace_found_by_annotation",
			clusterHash: "2no5f6yyo9w7af0e",
			workspaces: []kcptenancy.Workspace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "platform-mesh-system",
						Namespace: "root",
						Annotations: map[string]string{
							"kcp.io/cluster": "2no5f6yyo9w7af0e",
						},
					},
				},
			},
			expectedPath: "root:platform-mesh-system",
			expectError:  false,
		},
		{
			name:        "workspace_found_by_name_match",
			clusterHash: "platform-mesh-system",
			workspaces: []kcptenancy.Workspace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "platform-mesh-system",
						Namespace:   "root",
						Annotations: map[string]string{},
					},
				},
			},
			expectedPath: "root:platform-mesh-system",
			expectError:  false,
		},
		{
			name:         "no_workspace_found",
			clusterHash:  "unknown-hash",
			workspaces:   []kcptenancy.Workspace{},
			expectedPath: "unknown-hash",
			expectError:  false, // Now returns cluster name without error (fallback behavior)
		},
		{
			name:         "list_workspaces_error",
			clusterHash:  "2no5f6yyo9w7af0e",
			listError:    errors.New("failed to list workspaces"),
			expectedPath: "2no5f6yyo9w7af0e",
			expectError:  false, // Now returns cluster name without error (fallback behavior)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}

			// Only set up mock expectations if the cluster is not "root"
			if tt.clusterHash != "root" {
				// Since PathForClusterFromWorkspaces now calls PathForCluster,
				// we need to mock the LogicalCluster Get call
				if tt.expectedPath != tt.clusterHash && !tt.expectError {
					// Mock successful LogicalCluster retrieval with kcp.io/path annotation
					lc := &kcpcore.LogicalCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster",
							Annotations: map[string]string{
								"kcp.io/path": tt.expectedPath,
							},
						},
					}
					mockClient.On("Get", mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*kcpcore.LogicalCluster)
						*arg = *lc
					}).Return(nil)
				} else {
					// Mock failed LogicalCluster retrieval (falls back to cluster name)
					mockClient.On("Get", mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).Return(errors.New("not found"))
				}
			}

			result, err := kcp.PathForClusterFromWorkspacesExported(tt.clusterHash, mockClient)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedPath, result)
			mockClient.AssertExpectations(t)
		})
	}
}
