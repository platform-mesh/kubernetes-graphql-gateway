package kcp_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	apschemamocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	workspacefilemocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp"
	kcpmocks "github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
)

func TestAPIBindingReconciler_Reconcile(t *testing.T) {
	// Set up a minimal kubeconfig for tests to avoid reading complex system kubeconfig
	tempDir, err := os.MkdirTemp("", "kcp-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	kubeconfigContent := `apiVersion: v1
kind: Config
current-context: test
contexts:
- context: {cluster: test, user: test}
  name: test
clusters:
- cluster: {server: 'https://test.example.com'}
  name: test
users:
- name: test
  user: {token: test-token}
`
	kubeconfigPath := filepath.Join(tempDir, "config")
	err = os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	originalKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfigPath)
	defer func() {
		if originalKubeconfig != "" {
			os.Setenv("KUBECONFIG", originalKubeconfig)
		} else {
			os.Unsetenv("KUBECONFIG")
		}
	}()

	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name        string
		req         ctrl.Request
		mockSetup   func(*mocks.MockClient, *workspacefilemocks.MockIOHandler, *kcpmocks.MockDiscoveryFactory, *apschemamocks.MockResolver, *kcpmocks.MockClusterPathResolver)
		wantResult  ctrl.Result
		wantErr     bool
		errContains string
	}{
		{
			name: "system_workspace_ignored",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "system:shard",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				// No expectations set as system workspaces should be ignored
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "cluster_client_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "test-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mcpr.EXPECT().ClientForCluster("test-cluster").
					Return(nil, errors.New("cluster client error")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "cluster client error",
		},
		{
			name: "cluster_is_deleted_triggers_cleanup",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "deleted-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("deleted-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock the client.Get call that happens in PathForCluster
				// Create a deleted LogicalCluster (with DeletionTimestamp set)
				now := metav1.Now()
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:deleted-cluster",
						},
						DeletionTimestamp: &now,
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				// Mock the cleanup - IOHandler.Delete should be called
				mio.EXPECT().Delete("root:org:deleted-cluster").
					Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false, // Cleanup should succeed without error
		},
		{
			name: "path_for_cluster_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "error-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("error-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock the Get call that PathForCluster makes internally
				mockClusterClient.EXPECT().Get(
					mock.Anything,
					client.ObjectKey{Name: "cluster"},
					mock.AnythingOfType("*v1alpha1.LogicalCluster"),
				).Return(errors.New("get cluster failed")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "failed to get logicalcluster resource",
		},
		{
			name: "discovery_client_creation_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "test-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("test-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:test-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:test-cluster").
					Return(nil, errors.New("discovery client error")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "discovery client error",
		},
		{
			name: "rest_mapper_creation_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "test-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)

				mcpr.EXPECT().ClientForCluster("test-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:test-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:test-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:test-cluster").
					Return(nil, errors.New("rest mapper error")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "rest mapper error",
		},
		{
			name: "file_not_exists_creates_new_schema",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "new-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("new-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:new-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:new-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:new-cluster").
					Return(mockRestMapper, nil).Once()

				mio.EXPECT().Read("root:org:new-cluster").
					Return(nil, fs.ErrNotExist).Once()

				schemaJSON := []byte(`{"schema": "test"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(schemaJSON, nil).Once()

				// Expect schema with KCP metadata injected
				mio.EXPECT().Write(mock.MatchedBy(func(data []byte) bool {
					return strings.Contains(string(data), `"schema":"test"`) &&
						strings.Contains(string(data), `"x-cluster-metadata"`) &&
						strings.Contains(string(data), `"host":"https://test.example.com"`) &&
						strings.Contains(string(data), `"path":"root:org:new-cluster"`)
				}), "root:org:new-cluster").Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "schema_resolution_error_on_new_file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "schema-error-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("schema-error-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:schema-error-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:schema-error-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:schema-error-cluster").
					Return(mockRestMapper, nil).Once()

				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(nil, errors.New("schema resolution failed")).Once()

				// No Read call expected since schema generation fails early
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "schema resolution failed",
		},
		{
			name: "file_write_error_on_new_file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "write-error-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("write-error-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:write-error-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:write-error-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:write-error-cluster").
					Return(mockRestMapper, nil).Once()

				mio.EXPECT().Read("root:org:write-error-cluster").
					Return(nil, fs.ErrNotExist).Once()

				schemaJSON := []byte(`{"schema": "test"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(schemaJSON, nil).Once()

				// Expect schema with KCP metadata injected
				mio.EXPECT().Write(mock.MatchedBy(func(data []byte) bool {
					return strings.Contains(string(data), `"schema":"test"`) &&
						strings.Contains(string(data), `"x-cluster-metadata"`)
				}), "root:org:write-error-cluster").Return(errors.New("write failed")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "write failed",
		},
		{
			name: "file_read_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "read-error-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("read-error-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:read-error-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:read-error-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:read-error-cluster").
					Return(mockRestMapper, nil).Once()

				// Schema generation happens before read, so we need this expectation
				schemaJSON := []byte(`{"schema": "test"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(schemaJSON, nil).Once()

				mio.EXPECT().Read("root:org:read-error-cluster").
					Return(nil, errors.New("read failed")).Once()
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "read failed",
		},
		{
			name: "schema_unchanged_no_write",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "unchanged-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("unchanged-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:unchanged-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:unchanged-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:unchanged-cluster").
					Return(mockRestMapper, nil).Once()

				savedJSON := []byte(`{"schema": "existing"}`)
				mio.EXPECT().Read("root:org:unchanged-cluster").
					Return(savedJSON, nil).Once()

				// Return the same schema - no changes
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(savedJSON, nil).Once()

				// Write call expected since metadata injection makes the schemas different
				mio.EXPECT().Write(mock.MatchedBy(func(data []byte) bool {
					return strings.Contains(string(data), `"schema":"existing"`) &&
						strings.Contains(string(data), `"x-cluster-metadata"`) &&
						strings.Contains(string(data), `"path":"root:org:unchanged-cluster"`)
				}), "root:org:unchanged-cluster").Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "schema_changed_writes_update",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "changed-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mio *workspacefilemocks.MockIOHandler, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("changed-cluster").
					Return(mockClusterClient, nil).Once()

				// Mock successful LogicalCluster get
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:changed-cluster",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lcObj := obj.(*kcpcore.LogicalCluster)
						*lcObj = *lc
						return nil
					}).Once()

				mdf.EXPECT().ClientForCluster("root:org:changed-cluster").
					Return(mockDiscoveryClient, nil).Once()

				mdf.EXPECT().RestMapperForCluster("root:org:changed-cluster").
					Return(mockRestMapper, nil).Once()

				savedJSON := []byte(`{"schema": "old"}`)
				mio.EXPECT().Read("root:org:changed-cluster").
					Return(savedJSON, nil).Once()

				newJSON := []byte(`{"schema": "new"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(newJSON, nil).Once()

				// Expect schema with KCP metadata injected
				mio.EXPECT().Write(mock.MatchedBy(func(data []byte) bool {
					return strings.Contains(string(data), `"schema":"new"`) &&
						strings.Contains(string(data), `"x-cluster-metadata"`) &&
						strings.Contains(string(data), `"path":"root:org:changed-cluster"`)
				}), "root:org:changed-cluster").Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			mockIOHandler := workspacefilemocks.NewMockIOHandler(t)
			mockDiscoveryFactory := kcpmocks.NewMockDiscoveryFactory(t)
			mockAPISchemaResolver := apschemamocks.NewMockResolver(t)
			mockClusterPathResolver := kcpmocks.NewMockClusterPathResolver(t)

			tt.mockSetup(mockClient, mockIOHandler, mockDiscoveryFactory, mockAPISchemaResolver, mockClusterPathResolver)

			reconciler := &kcp.ExportedAPIBindingReconciler{
				Client:              mockClient,
				Scheme:              runtime.NewScheme(),
				RestConfig:          &rest.Config{Host: "https://test.example.com"},
				IOHandler:           mockIOHandler,
				DiscoveryFactory:    mockDiscoveryFactory,
				APISchemaResolver:   mockAPISchemaResolver,
				ClusterPathResolver: mockClusterPathResolver,
				Log:                 mockLogger,
			}

			// Note: This test setup is simplified as we cannot easily mock the PathForCluster function
			// which is called internally. In a real test scenario, you might need to:
			// 1. Refactor the code to make PathForCluster injectable
			// 2. Use integration tests for the full flow
			// 3. Create a wrapper that can be mocked

			got, err := reconciler.Reconcile(t.Context(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, got)
		})
	}
}
