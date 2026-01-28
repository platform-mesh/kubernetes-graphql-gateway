package kcp_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	apschemamocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	kcpmocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

func TestAPIBindingReconciler_Reconcile(t *testing.T) {
	// Set up a minimal kubeconfig for tests to avoid reading complex system kubeconfig
	tempDir, err := os.MkdirTemp("", "kcp-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck

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
	os.Setenv("KUBECONFIG", kubeconfigPath) //nolint:errcheck
	defer func() {
		if originalKubeconfig != "" {
			os.Setenv("KUBECONFIG", originalKubeconfig) //nolint:errcheck
		} else {
			os.Unsetenv("KUBECONFIG") //nolint:errcheck
		}
	}()

	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name        string
		req         ctrl.Request
		mockSetup   func(*mocks.MockClient, *kcpmocks.MockDiscoveryFactory, *apschemamocks.MockResolver, *kcpmocks.MockClusterPathResolver)
		setupFS     func(schemaDir string, t *testing.T)
		postAssert  func(schemaDir string, t *testing.T)
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("system:shard").
					Return(mockClusterClient, nil).Maybe()
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("deleted-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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

				// No IO mock: file deletion verified via filesystem
			},
			setupFS: func(schemaDir string, t *testing.T) {
				_ = os.WriteFile(filepath.Join(schemaDir, "root:org:deleted-cluster"), []byte("data"), 0o644)
			},
			postAssert: func(schemaDir string, t *testing.T) {
				_, err := os.Stat(filepath.Join(schemaDir, "root:org:deleted-cluster"))
				assert.Error(t, err)
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("error-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("test-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)

				mcpr.EXPECT().ClientForCluster("test-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("new-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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

				schemaJSON := []byte(`{"schema": "test"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(schemaJSON, nil).Once()
			},
			postAssert: func(schemaDir string, t *testing.T) {
				data, err := os.ReadFile(filepath.Join(schemaDir, "root:org:new-cluster"))
				assert.NoError(t, err)
				s := string(data)
				assert.Contains(t, s, `"schema":"test"`)
				assert.Contains(t, s, `"x-cluster-metadata"`)
				assert.Contains(t, s, `"host":"https://test.example.com"`)
				assert.Contains(t, s, `"path":"root:org:new-cluster"`)
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("schema-error-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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
			name: "file_read_error",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "read-error-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("read-error-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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
			},
			setupFS: func(schemaDir string, t *testing.T) {
				p := filepath.Join(schemaDir, "root:org:read-error-cluster")
				_ = os.WriteFile(p, []byte("existing"), 0o000) // unreadable
			},
			wantResult:  ctrl.Result{},
			wantErr:     true,
			errContains: "failed to read JSON file",
		},
		{
			name: "schema_unchanged_no_write",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "unchanged-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("unchanged-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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

				// Return the same schema - no changes
				savedJSON := []byte(`{"schema": "existing"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(savedJSON, nil).Once()
			},
			setupFS: func(schemaDir string, t *testing.T) {
				_ = os.WriteFile(filepath.Join(schemaDir, "root:org:unchanged-cluster"), []byte(`{"schema": "existing"}`), 0o644)
			},
			postAssert: func(schemaDir string, t *testing.T) {
				data, err := os.ReadFile(filepath.Join(schemaDir, "root:org:unchanged-cluster"))
				assert.NoError(t, err)
				s := string(data)
				assert.Contains(t, s, `"schema":"existing"`)
				assert.Contains(t, s, `"x-cluster-metadata"`)
				assert.Contains(t, s, `"path":"root:org:unchanged-cluster"`)
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
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("changed-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						*obj.(*kcpapis.APIBinding) = *ab
						return nil
					}).Once()

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

				newJSON := []byte(`{"schema": "new"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(newJSON, nil).Once()
			},
			setupFS: func(schemaDir string, t *testing.T) {
				_ = os.WriteFile(filepath.Join(schemaDir, "root:org:changed-cluster"), []byte(`{"schema": "old"}`), 0o644)
			},
			postAssert: func(schemaDir string, t *testing.T) {
				data, err := os.ReadFile(filepath.Join(schemaDir, "root:org:changed-cluster"))
				assert.NoError(t, err)
				s := string(data)
				assert.Contains(t, s, `"schema":"new"`)
				assert.Contains(t, s, `"x-cluster-metadata"`)
				assert.Contains(t, s, `"path":"root:org:changed-cluster"`)
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "remove_initializer_from_apibinding",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-binding"},
				ClusterName:    "new-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				mockClusterClient := mocks.NewMockClient(t)
				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mcpr.EXPECT().ClientForCluster("new-cluster").
					Return(mockClusterClient, nil).Once()

				ab := &kcpapis.APIBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
						Annotations: map[string]string{
							"initializer.apis.kcp.io/gateway": "true",
						},
					},
				}
				mockClusterClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-binding"}, mock.AnythingOfType("*v1alpha1.APIBinding")).
					RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						abObj := obj.(*kcpapis.APIBinding)
						*abObj = *ab
						return nil
					}).Once()

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

				schemaJSON := []byte(`{"schema": "test"}`)
				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).
					Return(schemaJSON, nil).Once()

				mockClusterClient.EXPECT().Update(mock.Anything, mock.MatchedBy(func(obj *kcpapis.APIBinding) bool {
					_, exists := obj.Annotations["initializer.apis.kcp.io/gateway"]
					return !exists
				})).Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			mockDiscoveryFactory := kcpmocks.NewMockDiscoveryFactory(t)
			mockAPISchemaResolver := apschemamocks.NewMockResolver(t)
			mockClusterPathResolver := kcpmocks.NewMockClusterPathResolver(t)

			tt.mockSetup(mockClient, mockDiscoveryFactory, mockAPISchemaResolver, mockClusterPathResolver)

			schemaDir := t.TempDir()
			if tt.setupFS != nil {
				tt.setupFS(schemaDir, t)
			}
			fh, ferr := workspacefile.NewIOHandler(schemaDir)
			assert.NoError(t, ferr)

			reconciler := &kcp.ExportedAPIBindingReconciler{
				Client:              mockClient,
				Scheme:              runtime.NewScheme(),
				RestConfig:          &rest.Config{Host: "https://test.example.com"},
				IOHandler:           fh,
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
			if tt.postAssert != nil {
				tt.postAssert(schemaDir, t)
			}
		})
	}
}
