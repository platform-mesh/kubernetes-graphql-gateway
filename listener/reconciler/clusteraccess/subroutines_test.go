package clusteraccess

import (
	"context"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	workspacefile_mocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenerateSchemaSubroutine_Process_InvalidResourceType(t *testing.T) {
	mockIO := workspacefile_mocks.NewMockIOHandler(t)
	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: mockIO,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	_, opErr := s.Process(context.Background(), &metav1.PartialObjectMetadata{})

	assert.NotNil(t, opErr)
}

func TestGenerateSchemaSubroutine_Process_MissingHostReturnsError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = gatewayv1alpha1.AddToScheme(scheme)

	ca := &gatewayv1alpha1.ClusterAccess{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Annotations: map[string]string{},
		},
		Spec: gatewayv1alpha1.ClusterAccessSpec{
			// Host is intentionally empty to trigger validation error
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ca).Build()

	mockIO := workspacefile_mocks.NewMockIOHandler(t)
	mockIO.EXPECT().Write(mock.Anything, mock.Anything).Maybe().Return(nil)
	mockIO.EXPECT().Delete(mock.Anything).Maybe().Return(nil)

	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: mockIO,
		log:       log,
		opts: reconciler.ReconcilerOpts{
			Client:      fakeClient,
			Config:      &rest.Config{Host: "https://unit-test.invalid"},
			ManagerOpts: ctrl.Options{Scheme: scheme},
			Scheme:      scheme,
		},
	}
	s := &generateSchemaSubroutine{reconciler: r}

	res, opErr := s.Process(context.Background(), ca)

	assert.NotNil(t, opErr)
	assert.Equal(t, ctrl.Result{}, res)
}

func TestGenerateSchemaSubroutine_Finalize_DeletesCurrentAndPreviousPaths(t *testing.T) {
	mockIO := workspacefile_mocks.NewMockIOHandler(t)
	log, _ := logger.New(logger.DefaultConfig())

	// Expect deletion of both current and previous paths
	mockIO.EXPECT().Delete("current-path").Return(nil).Once()
	mockIO.EXPECT().Delete("previous-path").Return(nil).Once()

	r := &ClusterAccessReconciler{
		ioHandler: mockIO,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	ca := &gatewayv1alpha1.ClusterAccess{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-resource",
		},
		Spec: gatewayv1alpha1.ClusterAccessSpec{
			Path: "current-path",
		},
		Status: gatewayv1alpha1.ClusterAccessStatus{
			ObservedPath: "previous-path",
		},
	}

	res, opErr := s.Finalize(context.Background(), ca)

	assert.Nil(t, opErr)
	assert.Equal(t, ctrl.Result{}, res)
}

func TestGenerateSchemaSubroutine_restMapperFromConfig_SucceedsWithMinimalConfig(t *testing.T) {
	mockIO := workspacefile_mocks.NewMockIOHandler(t)
	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: mockIO,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	rm, err := s.restMapperFromConfig(&rest.Config{})

	assert.NotNil(t, rm)
	assert.NoError(t, err)
}
