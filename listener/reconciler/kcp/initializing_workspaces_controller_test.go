package kcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	apschemamocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	kcpmocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	kcpcore "github.com/kcp-dev/sdk/apis/core/v1alpha1"
)

func TestInitializingWorkspacesReconciler_Reconcile(t *testing.T) {
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
		clusterName string
		mockSetup   func(*mocks.MockClient, *kcpmocks.MockDiscoveryFactory, *apschemamocks.MockResolver, *kcpmocks.MockClusterPathResolver)
		wantResult  ctrl.Result
		wantErr     bool
	}{
		{
			name: "system_workspace_ignored",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-cluster"},
			},
			clusterName: "system:shard",
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "successful_initialization",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-cluster"},
			},
			clusterName: "root:org",
			mockSetup: func(mc *mocks.MockClient, mdf *kcpmocks.MockDiscoveryFactory, mar *apschemamocks.MockResolver, mcpr *kcpmocks.MockClusterPathResolver) {
				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						Annotations: map[string]string{
							"kcp.io/path": "root:org:test-cluster",
						},
					},
					Spec: kcpcore.LogicalClusterSpec{
						Initializers: []kcpcore.LogicalClusterInitializer{
							kcpcore.LogicalClusterInitializer(common.GatewayInitializer),
						},
					},
				}

				mc.EXPECT().Get(mock.Anything, types.NamespacedName{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcpcore.LogicalCluster) = *lc
					}).Return(nil).Once()

				mockDiscoveryClient := kcpmocks.NewMockDiscoveryInterface(t)
				mockRestMapper := kcpmocks.NewMockRESTMapper(t)

				mdf.EXPECT().ClientForCluster("root:org:test-cluster").Return(mockDiscoveryClient, nil).Once()
				mdf.EXPECT().RestMapperForCluster("root:org:test-cluster").Return(mockRestMapper, nil).Once()

				mar.EXPECT().Resolve(mockDiscoveryClient, mockRestMapper).Return([]byte(`{"schema": "test"}`), nil).Once()

				parentClient := mocks.NewMockClient(t)
				mcpr.EXPECT().ClientForCluster("root:org").Return(parentClient, nil).Once()

				parentClient.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcpcore.LogicalCluster) = *lc
					}).Return(nil).Once()

				parentClient.EXPECT().Update(mock.Anything, mock.MatchedBy(func(obj *kcpcore.LogicalCluster) bool {
					return len(obj.Spec.Initializers) == 0
				})).Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := mocks.NewMockClient(t)
			mdf := kcpmocks.NewMockDiscoveryFactory(t)
			mar := apschemamocks.NewMockResolver(t)
			mcpr := kcpmocks.NewMockClusterPathResolver(t)

			tt.mockSetup(mc, mdf, mar, mcpr)

			schemaDir := t.TempDir()
			fh, err := workspacefile.NewIOHandler(schemaDir)
			assert.NoError(t, err)

			r := &kcp.ExportedInitializingWorkspacesReconciler{
				Client:              mc,
				DiscoveryFactory:    mdf,
				APISchemaResolver:   mar,
				ClusterPathResolver: mcpr,
				IOHandler:           fh,
				Log:                 mockLogger,
			}

			result, err := r.Reconcile(mccontext.WithCluster(context.Background(), tt.clusterName), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				if !assert.NoError(t, err) {
					t.FailNow()
				}
			}
			assert.Equal(t, tt.wantResult, result)

			if tt.name == "successful_initialization" {
				_, err := os.Stat(filepath.Join(schemaDir, "root:org:test-cluster"))
				assert.NoError(t, err)
			}
		})
	}
}
