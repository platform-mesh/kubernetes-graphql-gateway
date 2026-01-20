package kcp_test

import (
	"context"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	kcpmocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
)

func TestLogicalClusterReconciler_Reconcile(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name       string
		req        ctrl.Request
		mockSetup  func(*mocks.MockClient, *kcpmocks.MockClusterPathResolver)
		wantResult ctrl.Result
		wantErr    bool
	}{
		{
			name: "system_workspace_ignored",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-cluster"},
				ClusterName:    "system:shard",
			},
			mockSetup:  func(mc *mocks.MockClient, mcpr *kcpmocks.MockClusterPathResolver) {},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "remove_initializer_account_workspace",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-cluster"},
				ClusterName:    "test-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mcpr *kcpmocks.MockClusterPathResolver) {
				mcpr.EXPECT().ClientForCluster("test-cluster").Return(mc, nil).Once()

				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: kcpcore.LogicalClusterSpec{
						Initializers: []kcpcore.LogicalClusterInitializer{
							kcpcore.LogicalClusterInitializer(common.GatewayInitializer),
							"other-initializer",
						},
					},
				}

				mc.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcpcore.LogicalCluster) = *lc
					}).Return(nil).Once()

				ws := &kcptenancy.Workspace{
					Spec: kcptenancy.WorkspaceSpec{
						Type: &kcptenancy.WorkspaceTypeReference{
							Name: "account",
						},
					},
				}
				mc.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.Workspace")).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcptenancy.Workspace) = *ws
					}).Return(nil).Once()

				mc.EXPECT().Update(mock.Anything, mock.MatchedBy(func(obj *kcpcore.LogicalCluster) bool {
					return len(obj.Spec.Initializers) == 1 && string(obj.Spec.Initializers[0]) == "other-initializer"
				})).Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "skip_initializer_non_account_workspace",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-cluster"},
				ClusterName:    "test-cluster",
			},
			mockSetup: func(mc *mocks.MockClient, mcpr *kcpmocks.MockClusterPathResolver) {
				mcpr.EXPECT().ClientForCluster("test-cluster").Return(mc, nil).Once()

				lc := &kcpcore.LogicalCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: kcpcore.LogicalClusterSpec{
						Initializers: []kcpcore.LogicalClusterInitializer{
							kcpcore.LogicalClusterInitializer(common.GatewayInitializer),
						},
					},
				}

				mc.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.LogicalCluster")).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcpcore.LogicalCluster) = *lc
					}).Return(nil).Once()

				ws := &kcptenancy.Workspace{
					Spec: kcptenancy.WorkspaceSpec{
						Type: &kcptenancy.WorkspaceTypeReference{
							Name: "universal",
						},
					},
				}
				mc.EXPECT().Get(mock.Anything, client.ObjectKey{Name: "test-cluster"}, mock.AnythingOfType("*v1alpha1.Workspace")).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						*obj.(*kcptenancy.Workspace) = *ws
					}).Return(nil).Once()
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := mocks.NewMockClient(t)
			mcpr := kcpmocks.NewMockClusterPathResolver(t)
			tt.mockSetup(mc, mcpr)

			r := &kcp.LogicalClusterReconciler{
				Client:              mc,
				ClusterPathResolver: mcpr,
				Log:                 mockLogger,
			}

			result, err := r.Reconcile(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
			}
		})
	}
}
