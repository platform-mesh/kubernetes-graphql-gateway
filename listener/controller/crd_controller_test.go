package controller_test

import (
	"context"
	"errors"
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/controller"
	workspacefileMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile/mocks"

	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mockCRDResolver struct {
	mock.Mock
}

func (m *mockCRDResolver) Resolve() ([]byte, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCRDResolver) ResolveApiSchema(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	args := m.Called(crd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

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
			name:    "get error",
			getErr:  errors.New("get-error"),
			readErr: nil,
			wantErr: controller.ErrGetReconciledObj,
		},
		{
			name:    "not found read error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: errors.New("read-error"),
			wantErr: controller.ErrReadJSON,
		},
		{
			name:    "not found resolve error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: nil,
			wantErr: controller.ErrResolveSchema,
		},
		{
			name:    "not found write error",
			getErr:  apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "crds"}, "my-crd"),
			readErr: nil,
			wantErr: controller.ErrWriteJSON,
		},
		{
			name:    "successful update",
			getErr:  nil,
			readErr: nil,
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ioHandler := workspacefileMocks.NewMockIOHandler(t)
			fakeClient := mocks.NewMockClient(t)
			crdResolver := &mockCRDResolver{}

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
						crdResolver.On("Resolve").Return(nil, errors.New("resolve error"))
					} else if tc.wantErr == controller.ErrWriteJSON {
						crdResolver.On("Resolve").Return([]byte(`{"new":"schema"}`), nil)
						ioHandler.EXPECT().Write([]byte(`{"new":"schema"}`), "cluster1").Return(errors.New("write error"))
					} else {
						crdResolver.On("Resolve").Return([]byte("{}"), nil)
					}
				}
			} else if tc.getErr == nil {
				ioHandler.EXPECT().Read("cluster1").Return([]byte("{}"), nil)
				crdResolver.On("ResolveApiSchema", mock.Anything).Return([]byte(`{"new":"schema"}`), nil)
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
