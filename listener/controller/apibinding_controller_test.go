package controller_test

import (
	"context"
	"errors"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger/testlogger"
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

				controllerRuntimeClient.EXPECT().
					Get(mock.Anything, mock.Anything, mock.Anything).
					Return(nil).
					Run(func(
						ctx context.Context,
						key client.ObjectKey,
						obj client.Object,
						opts ...client.GetOption,
					) {
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
		{
			name:        "workspace_delete_error",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)
				controllerRuntimeClient.EXPECT().
					Get(mock.Anything, mock.Anything, mock.Anything).
					Return(nil).
					Run(func(
						ctx context.Context,
						key client.ObjectKey,
						obj client.Object,
						opts ...client.GetOption,
					) {
						lc := obj.(*kcpcore.LogicalCluster)
						lc.Annotations = map[string]string{
							"kcp.io/path": "dev-cluster",
						}
						lc.DeletionTimestamp = &metav1.Time{}
					})
				ioHandler.EXPECT().Delete(mock.Anything).Return(assert.AnError)
			},
			err: assert.AnError,
		},
		{
			name:        "missing_annotation_error",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)
				controllerRuntimeClient.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			err: errors.New("failed to get cluster path from kcp.io/path annotation"),
		},
		{
			name:        "nil_annotation_error",
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
						// Do not set Annotations (nil)
					})
			},
			err: errors.New("failed to get cluster path from kcp.io/path annotation"),
		},
		{
			name:        "empty_annotation_error",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)
				controllerRuntimeClient.EXPECT().
					Get(mock.Anything, mock.Anything, mock.Anything).
					Return(nil).
					Run(func(
						ctx context.Context,
						key client.ObjectKey,
						obj client.Object,
						opts ...client.GetOption,
					) {
						lc := obj.(*kcpcore.LogicalCluster)
						lc.Annotations = map[string]string{
							"kcp.io/path": "",
						}
					})
				ioHandler.EXPECT().Read(mock.Anything).Return(nil, nil)
				discoverFactory.EXPECT().RestMapperForCluster(mock.Anything).Return(nil, nil)
				discoverFactory.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil)
				apiSchemaResolver.EXPECT().Resolve(mock.Anything, mock.Anything).Return(nil, nil)
			},
			err: nil,
		},
		{
			name:        "logicalcluster_get_error",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)
				controllerRuntimeClient.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
			},
			err: assert.AnError,
		},
		{
			name:        "client_for_cluster_error",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(nil, assert.AnError)
			},
			err: assert.AnError,
		},
		{
			name:        "successful_schema_update",
			clusterName: "dev-cluster",
			mockSetup: func(
				ioHandler *workspacefileMocks.MockIOHandler,
				discoverFactory *discoveryclientMocks.MockFactory,
				apiSchemaResolver *apischemaMocks.MockResolver,
				clusterPathResolver *clusterpathMocks.MockResolver,
			) {
				controllerRuntimeClient := &controllerRuntimeMocks.MockClient{}
				clusterPathResolver.EXPECT().ClientForCluster("dev-cluster").Return(controllerRuntimeClient, nil)
				controllerRuntimeClient.EXPECT().
					Get(mock.Anything, mock.Anything, mock.Anything).
					Return(nil).
					Run(func(
						ctx context.Context,
						key client.ObjectKey,
						obj client.Object,
						opts ...client.GetOption,
					) {
						lc := obj.(*kcpcore.LogicalCluster)
						lc.Annotations = map[string]string{
							"kcp.io/path": "dev-cluster",
						}
					})
				ioHandler.EXPECT().Read("dev-cluster").Return([]byte("{}"), nil)
				discoverFactory.EXPECT().RestMapperForCluster("dev-cluster").Return(nil, nil)
				discoverFactory.EXPECT().ClientForCluster("dev-cluster").Return(nil, nil)
				apiSchemaResolver.EXPECT().Resolve(nil, nil).Return([]byte(`{"new":"schema"}`), nil)
				ioHandler.EXPECT().Write([]byte(`{"new":"schema"}`), "dev-cluster").Return(nil)
			},
			err: nil,
		},
	}

	log := testlogger.New().HideLogOutput().Logger
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ioHandler := workspacefileMocks.NewMockIOHandler(t)
			discoverFactory := discoveryclientMocks.
				NewMockFactory(t)
			apiSchemaResolver := apischemaMocks.
				NewMockResolver(t)
			clusterPathResolver := clusterpathMocks.
				NewMockResolver(t)

			if tt.mockSetup != nil {
				tt.mockSetup(
					ioHandler,
					discoverFactory,
					apiSchemaResolver,
					clusterPathResolver,
				)
			}

			r := controller.NewAPIBindingReconciler(
				ioHandler,
				discoverFactory,
				apiSchemaResolver,
				clusterPathResolver,
				log,
			)
			_, err := r.Reconcile(context.Background(), ctrl.Request{ClusterName: tt.clusterName})

			if tt.name == "logicalcluster_get_error" {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.Equal(t, tt.err, err)
			}
		})
	}
}
