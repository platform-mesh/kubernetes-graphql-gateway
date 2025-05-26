package controller_test

import (
	"context"
	"errors"
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/controller"
	controllerMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/controller/mocks"
	workspacefileMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile/mocks"

	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestCRDReconciler tests the CRDReconciler's Reconcile method.
// It checks if the method handles different scenarios correctly, including
// errors when getting the CRD and reading the JSON schema.
func TestCRDReconciler(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger
	type scenario struct {
		name    string
		getErr  error
		readErr error
		wantErr error
	}
	tests := []scenario{
		{
			name:    "get_error",
			getErr:  errors.New("get-error"),
			readErr: nil,
			wantErr: controller.ErrGetReconciledObj,
		},
		{
			name:    "not_found_read_error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: errors.New("read-error"),
			wantErr: controller.ErrReadJSON,
		},
		{
			name:    "not_found_resolve_error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: nil,
			wantErr: controller.ErrResolveSchema,
		},
		{
			name:    "not_found_write_error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: nil,
			wantErr: controller.ErrWriteJSON,
		},
		{
			name:    "successful_update",
			getErr:  nil,
			readErr: nil,
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ioHandler := workspacefileMocks.NewMockIOHandler(t)
			fakeClient := mocks.NewMockClient(t)
			crdResolver := controllerMocks.NewMockCRDResolver(t)

			r := controller.NewCRDReconciler(
				"cluster1",
				fakeClient,
				crdResolver,
				ioHandler,
				log,
			)

			req := reconcile.Request{NamespacedName: client.ObjectKey{Name: "my-crd"}}
			fakeClient.EXPECT().Get(
				mock.Anything,
				req.NamespacedName,
				mock.Anything,
			).Return(tc.getErr)

			if apierrors.IsNotFound(tc.getErr) {
				ioHandler.EXPECT().Read("cluster1").Return([]byte("{}"), tc.readErr)
				if tc.readErr == nil {
					if tc.wantErr == controller.ErrResolveSchema {
						crdResolver.EXPECT().Resolve().Return(nil, errors.New("resolve error"))
					} else if tc.wantErr == controller.ErrWriteJSON {
						crdResolver.EXPECT().Resolve().Return([]byte(`{"new":"schema"}`), nil)
						ioHandler.EXPECT().
							Write([]byte(`{"new":"schema"}`), "cluster1").
							Return(errors.New("write error"))
					} else {
						crdResolver.EXPECT().Resolve().Return([]byte("{}"), nil)
					}
				}
			} else if tc.getErr == nil {
				ioHandler.EXPECT().Read("cluster1").Return([]byte("{}"), nil)
				crdResolver.EXPECT().ResolveApiSchema(mock.Anything).Return([]byte(`{"new":"schema"}`), nil)
				ioHandler.EXPECT().Write([]byte(`{"new":"schema"}`), "cluster1").Return(nil)
			}

			_, err := r.Reconcile(context.Background(), req)
			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
