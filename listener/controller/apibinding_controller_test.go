package controller_test

import (
	"context"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	controllerRuntimeMocks "github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver/mocks"
	apischemaMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/apischema/mocks"
	clusterpathMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/clusterpath/mocks"
	discoveryclientMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient/mocks"
	workspacefileMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile/mocks"
)

func TestAPIBindingReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		mockSetup   func(
			io *workspacefileMocks.MockIOHandler,
			df *discoveryclientMocks.MockFactory,
			sc *apischemaMocks.MockResolver,
			pr *clusterpathMocks.MockResolver,
		)
		err error
	}{
		{
			name:        "workspace_is_deleted_ERROR",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)

				controllerRuntimeClient.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil).
					Run(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) {
						lc := obj.(*kcpcore.LogicalCluster) // Get the pointer argument
						lc.Annotations = map[string]string{
							"kcp.io/path": "dev-cluster",
						}
						lc.DeletionTimestamp = &metav1.Time{}
					})

				ioHandler.EXPECT().Delete(mock.Anything).Return(nil)
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ioHandler := workspacefileMocks.NewMockIOHandler(t)
			discoveryFactory := discoveryclientMocks.NewMockFactory(t)
			apiSchemaResolver := apischemaMocks.NewMockResolver(t)
			clusterPathResolver := clusterpathMocks.NewMockResolver(t)

			if tt.mockSetup != nil {
				tt.mockSetup(ioHandler, discoveryFactory, apiSchemaResolver, clusterPathResolver)
			}

			loggerCfg := logger.DefaultConfig()
			loggerCfg.Name = "test-apibinding-reconciler"
			log, err := logger.New(loggerCfg)
			assert.NoError(t, err)
			r := controller.NewAPIBindingReconciler(ioHandler, discoveryFactory, apiSchemaResolver, clusterPathResolver, log)
			_, err = r.Reconcile(context.Background(), ctrl.Request{ClusterName: tt.clusterName})
			assert.Equal(t, tt.err, err)
		})
	}
}
