package controller_test

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"

	schemamocks "github.com/openmfp/crd-gql-gateway/listener/apischema/mocks"
	"github.com/openmfp/crd-gql-gateway/listener/controller"
	discoverymocks "github.com/openmfp/crd-gql-gateway/listener/discoveryclient/mocks"
	iomocks "github.com/openmfp/crd-gql-gateway/listener/workspacefile/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	mockSchemaPath = "../apischema/mocks/schemas"
	mockSchema1    = "root_schema_mock_1.json"
	mockSchema2    = "root_schema_mock_2.json"
)

type testCase struct {
	name          string
	req           ctrl.Request
	expectError   bool
	expectRequeue bool
	ioMocks       func(*iomocks.MockIOHandler)
	dfMocks       func(*discoverymocks.MockFactory)
	schemaMocks   func(*schemamocks.MockResolver)
}

func TestReconcile(t *testing.T) {
	t.Skip()
	testCases := []testCase{
		{
			name: "should save root cluster API schema when an apibinding resource is created",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "root",
			},
			expectError:   false,
			expectRequeue: false,
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).Return(nil, os.ErrNotExist).Once()
				mh.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Once()
			},
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.EXPECT().Resolve(mock.Anything).RunAndReturn(
					func(_ discovery.DiscoveryInterface) ([]byte, error) {
						return os.ReadFile(path.Join(mockSchemaPath, mockSchema1))
					},
				)
			},
		},
		{
			name: "should skip reconciliation when a system workspace has been created",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "system:shard",
			},
			expectError:   false,
			expectRequeue: false,
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.AssertNotCalled(t, "Read")
				mh.AssertNotCalled(t, "Write")
			},
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.AssertNotCalled(t, "ClientForCluster")
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.AssertNotCalled(t, "Resolve")
			},
		},
		{
			name: "should return error when Discovery Client creation fails",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "0dv4pi71hgxq5jcm",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, errors.New("failed to parse rest config")).Once()
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.AssertNotCalled(t, "Read")
				mh.AssertNotCalled(t, "Write")
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.AssertNotCalled(t, "Resolve")
			},
		},
		{
			name: "should return error when schema resolution fails for a non-existing schema file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "0dv4pi71hgxq5jcm",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).Return(nil, os.ErrNotExist).Once()
				mh.AssertNotCalled(t, "Write")
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.EXPECT().Resolve(mock.Anything).Return(nil, errors.New("failed to get server preferred resources: unknown"))
			},
		},
		{
			name: "should return error when schema writing fails for a non-existing schema file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "root:sap:openmfp",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).Return(nil, os.ErrNotExist).Once()
				mh.EXPECT().Write(mock.Anything, mock.Anything).Return(os.ErrPermission)
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.EXPECT().Resolve(mock.Anything).RunAndReturn(
					func(_ discovery.DiscoveryInterface) ([]byte, error) {
						return os.ReadFile(path.Join(mockSchemaPath, mockSchema1))
					},
				).Once()
			},
		},
		{
			name: "should return error when schema resolution fails for an existing schema file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "0dv4pi71hgxq5jcm",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).RunAndReturn(
					func(_ string) ([]byte, error) {
						return os.ReadFile(path.Join(mockSchemaPath, mockSchema2))
					},
				).Once()
				mh.AssertNotCalled(t, "Write")
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.EXPECT().Resolve(mock.Anything).Return(nil, errors.New("failed to get server preferred resources: unknown"))
			},
		},
		{
			name: "should return error when schema writing fails for an existing schema file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "root:sap:openmfp",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.EXPECT().Resolve(mock.Anything).RunAndReturn(
					func(_ discovery.DiscoveryInterface) ([]byte, error) {
						return os.ReadFile(path.Join(mockSchemaPath, mockSchema1))
					},
				).Once()
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).RunAndReturn(
					func(_ string) ([]byte, error) {
						return os.ReadFile(path.Join(mockSchemaPath, mockSchema2))
					},
				).Once()
				mh.EXPECT().Write(mock.Anything, mock.Anything).Return(os.ErrPermission)
			},
		},
		{
			name: "should return error when schema reading fails with a random error for an existing schema file",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "tenancy.kcp.io",
				},
				ClusterName: "root:sap:openmfp",
			},
			expectError:   true,
			expectRequeue: false,
			dfMocks: func(mf *discoverymocks.MockFactory) {
				mf.EXPECT().ClientForCluster(mock.Anything).Return(nil, nil).Once()
			},
			schemaMocks: func(mr *schemamocks.MockResolver) {
				mr.AssertNotCalled(t, "Resolve")
			},
			ioMocks: func(mh *iomocks.MockIOHandler) {
				mh.EXPECT().Read(mock.Anything).Return(nil, os.ErrPermission).Once()
				mh.AssertNotCalled(t, "Write")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			reconciler := setupMocks(t, tc)

			res, err := reconciler.Reconcile(context.TODO(), tc.req)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expectRequeue, res.Requeue)
		})
	}
}

func setupMocks(t *testing.T, tc testCase) *controller.APIBindingReconciler {
	ioHandler := iomocks.NewMockIOHandler(t)
	if tc.ioMocks != nil {
		tc.ioMocks(ioHandler)
	}

	df := discoverymocks.NewMockFactory(t)
	if tc.dfMocks != nil {
		tc.dfMocks(df)
	}

	sc := schemamocks.NewMockResolver(t)
	if tc.schemaMocks != nil {
		tc.schemaMocks(sc)
	}

	return controller.NewAPIBindingReconciler(ioHandler, df, sc)
}
